package playbook

import (
	"testing"
)

func TestParseRaw(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantPlays int
		wantErr   bool
	}{
		{
			name: "single play",
			yaml: `
name: Test Play
hosts: localhost
tasks:
  - name: Say hello
    command:
      cmd: echo hello
`,
			wantPlays: 1,
			wantErr:   false,
		},
		{
			name: "multiple plays",
			yaml: `
- name: Play 1
  hosts: localhost
  tasks:
    - command:
        cmd: echo one

- name: Play 2
  hosts: localhost
  tasks:
    - command:
        cmd: echo two
`,
			wantPlays: 2,
			wantErr:   false,
		},
		{
			name:    "invalid yaml",
			yaml:    `{{{invalid`,
			wantErr: true,
		},
		{
			name: "play with vars",
			yaml: `
name: Test
hosts: localhost
vars:
  greeting: hello
  count: 5
tasks:
  - command:
      cmd: echo test
`,
			wantPlays: 1,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb, err := ParseRaw([]byte(tt.yaml), "test.yaml")
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if len(pb.Plays) != tt.wantPlays {
				t.Errorf("expected %d plays, got %d", tt.wantPlays, len(pb.Plays))
			}
		})
	}
}

func TestParseHostsFormats(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantHosts []string
	}{
		{
			name: "string host",
			yaml: `
hosts: myserver
tasks:
  - command:
      cmd: echo hello
`,
			wantHosts: []string{"myserver"},
		},
		{
			name: "list of hosts",
			yaml: `
hosts: [web1, web2, db1]
tasks:
  - command:
      cmd: echo hello
`,
			wantHosts: []string{"web1", "web2", "db1"},
		},
		{
			name: "block list of hosts",
			yaml: `
hosts:
  - app1
  - app2
tasks:
  - command:
      cmd: echo hello
`,
			wantHosts: []string{"app1", "app2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb, err := ParseRaw([]byte(tt.yaml), "test.yaml")
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			hosts := pb.Plays[0].Hosts
			if len(hosts) != len(tt.wantHosts) {
				t.Fatalf("expected %d hosts, got %d: %v", len(tt.wantHosts), len(hosts), hosts)
			}
			for i, h := range tt.wantHosts {
				if hosts[i] != h {
					t.Errorf("hosts[%d]: expected %q, got %q", i, h, hosts[i])
				}
			}
		})
	}
}

func TestParseRawTask(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		wantModule string
		wantParams map[string]any
	}{
		{
			name: "command with map params",
			yaml: `
hosts: localhost
tasks:
  - name: Test
    command:
      cmd: echo hello
`,
			wantModule: "command",
			wantParams: map[string]any{"cmd": "echo hello"},
		},
		{
			name: "file module",
			yaml: `
hosts: localhost
tasks:
  - file:
      path: /tmp/test
      state: directory
`,
			wantModule: "file",
			wantParams: map[string]any{"path": "/tmp/test", "state": "directory"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb, err := ParseRaw([]byte(tt.yaml), "test.yaml")
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if len(pb.Plays) == 0 || len(pb.Plays[0].Tasks) == 0 {
				t.Fatal("no tasks parsed")
			}
			task := pb.Plays[0].Tasks[0]
			if task.Module != tt.wantModule {
				t.Errorf("expected module %q, got %q", tt.wantModule, task.Module)
			}
			for k, v := range tt.wantParams {
				if task.Params[k] != v {
					t.Errorf("param %q: expected %v, got %v", k, v, task.Params[k])
				}
			}
		})
	}
}

func TestParseNotify(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		wantNotify []string
	}{
		{
			name: "single notify as string",
			yaml: `
hosts: localhost
tasks:
  - command:
      cmd: echo test
    notify: restart service
`,
			wantNotify: []string{"restart service"},
		},
		{
			name: "multiple notify as list",
			yaml: `
hosts: localhost
tasks:
  - command:
      cmd: echo test
    notify:
      - restart nginx
      - reload config
`,
			wantNotify: []string{"restart nginx", "reload config"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb, err := ParseRaw([]byte(tt.yaml), "test.yaml")
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			task := pb.Plays[0].Tasks[0]
			if len(task.Notify) != len(tt.wantNotify) {
				t.Errorf("expected %d notify handlers, got %d", len(tt.wantNotify), len(task.Notify))
				return
			}
			for i, n := range tt.wantNotify {
				if task.Notify[i] != n {
					t.Errorf("notify[%d]: expected %q, got %q", i, n, task.Notify[i])
				}
			}
		})
	}
}

func TestParseLoop(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantLoop int
	}{
		{
			name: "loop keyword",
			yaml: `
hosts: localhost
tasks:
  - command:
      cmd: echo {{ item }}
    loop:
      - one
      - two
      - three
`,
			wantLoop: 3,
		},
		{
			name: "with_items keyword",
			yaml: `
hosts: localhost
tasks:
  - command:
      cmd: echo {{ item }}
    with_items:
      - a
      - b
`,
			wantLoop: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb, err := ParseRaw([]byte(tt.yaml), "test.yaml")
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			task := pb.Plays[0].Tasks[0]
			if len(task.Loop) != tt.wantLoop {
				t.Errorf("expected loop length %d, got %d", tt.wantLoop, len(task.Loop))
			}
		})
	}
}

func TestExpandShorthand(t *testing.T) {
	tests := []struct {
		name       string
		task       *Task
		wantParams map[string]any
	}{
		{
			name: "key=value format",
			task: &Task{
				Module: "apt",
				Params: map[string]any{"_raw": "name=nginx state=present"},
			},
			wantParams: map[string]any{"name": "nginx", "state": "present"},
		},
		{
			name: "command single arg",
			task: &Task{
				Module: "command",
				Params: map[string]any{"_raw": "echo hello world"},
			},
			wantParams: map[string]any{"cmd": "echo hello world"},
		},
		{
			name: "file single arg",
			task: &Task{
				Module: "file",
				Params: map[string]any{"_raw": "/tmp/test"},
			},
			wantParams: map[string]any{"path": "/tmp/test"},
		},
		{
			name: "no expansion needed",
			task: &Task{
				Module: "command",
				Params: map[string]any{"cmd": "echo test"},
			},
			wantParams: map[string]any{"cmd": "echo test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ExpandShorthand(tt.task)
			for k, v := range tt.wantParams {
				if tt.task.Params[k] != v {
					t.Errorf("param %q: expected %v, got %v", k, v, tt.task.Params[k])
				}
			}
		})
	}
}

func TestParseHandlers(t *testing.T) {
	yaml := `
hosts: localhost
tasks:
  - name: Install nginx
    command:
      cmd: apt-get install nginx
    notify: restart nginx

handlers:
  - name: restart nginx
    command:
      cmd: systemctl restart nginx
`
	pb, err := ParseRaw([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(pb.Plays[0].Handlers) != 1 {
		t.Errorf("expected 1 handler, got %d", len(pb.Plays[0].Handlers))
	}

	handler := pb.Plays[0].Handlers[0]
	if handler.Name != "restart nginx" {
		t.Errorf("expected handler name 'restart nginx', got %q", handler.Name)
	}
}

func TestParseInclude(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		wantInclude string
		wantWhen    string
	}{
		{
			name: "simple include",
			yaml: `
hosts: localhost
tasks:
  - name: Setup docker
    include: https://example.com/docker-tasks.yaml
`,
			wantInclude: "https://example.com/docker-tasks.yaml",
		},
		{
			name: "include with when",
			yaml: `
hosts: localhost
tasks:
  - name: Setup from git
    include: git@github.com:user/repo.git//tasks/setup.yaml
    when: facts.os_family == 'Debian'
`,
			wantInclude: "git@github.com:user/repo.git//tasks/setup.yaml",
			wantWhen:    "facts.os_family == 'Debian'",
		},
		{
			name: "include with local path",
			yaml: `
hosts: localhost
tasks:
  - name: Include local tasks
    include: ./common/tasks.yaml
`,
			wantInclude: "./common/tasks.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb, err := ParseRaw([]byte(tt.yaml), "test.yaml")
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if len(pb.Plays) == 0 || len(pb.Plays[0].Tasks) == 0 {
				t.Fatal("no tasks parsed")
			}
			task := pb.Plays[0].Tasks[0]
			if task.Include != tt.wantInclude {
				t.Errorf("expected include %q, got %q", tt.wantInclude, task.Include)
			}
			if task.Module != "" {
				t.Errorf("expected empty module for include task, got %q", task.Module)
			}
			if tt.wantWhen != "" && task.When != tt.wantWhen {
				t.Errorf("expected when %q, got %q", tt.wantWhen, task.When)
			}
		})
	}
}

func TestParseIncludeMixedWithModules(t *testing.T) {
	yaml := `
hosts: localhost
tasks:
  - name: Create a file
    copy:
      dest: /tmp/test.txt
      content: "hello"
  - name: Include extras
    include: ./extras.yaml
  - name: Run command
    command:
      cmd: echo done
`
	pb, err := ParseRaw([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	tasks := pb.Plays[0].Tasks
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	// First task: module task
	if tasks[0].Module != "copy" {
		t.Errorf("task 0: expected module 'copy', got %q", tasks[0].Module)
	}
	if tasks[0].Include != "" {
		t.Errorf("task 0: expected empty include, got %q", tasks[0].Include)
	}

	// Second task: include task
	if tasks[1].Include != "./extras.yaml" {
		t.Errorf("task 1: expected include './extras.yaml', got %q", tasks[1].Include)
	}
	if tasks[1].Module != "" {
		t.Errorf("task 1: expected empty module, got %q", tasks[1].Module)
	}

	// Third task: module task
	if tasks[2].Module != "command" {
		t.Errorf("task 2: expected module 'command', got %q", tasks[2].Module)
	}
}

func TestParseIncludeTasks(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		wantInclude string
		wantVars    map[string]any
		wantWhen    string
	}{
		{
			name: "include_tasks basic",
			yaml: `
hosts: localhost
tasks:
  - name: Setup
    include_tasks: common/setup.yml
`,
			wantInclude: "common/setup.yml",
		},
		{
			name: "include_tasks with vars",
			yaml: `
hosts: localhost
tasks:
  - name: Install package
    include_tasks: install.yml
    vars:
      package_name: nginx
      version: "1.24"
`,
			wantInclude: "install.yml",
			wantVars:    map[string]any{"package_name": "nginx", "version": "1.24"},
		},
		{
			name: "include_tasks with when",
			yaml: `
hosts: localhost
tasks:
  - name: Debian setup
    include_tasks: debian.yml
    when: facts.os == "debian"
`,
			wantInclude: "debian.yml",
			wantWhen:    `facts.os == "debian"`,
		},
		{
			name: "include with vars (bare include)",
			yaml: `
hosts: localhost
tasks:
  - name: Install package
    include: install.yml
    vars:
      package_name: redis
`,
			wantInclude: "install.yml",
			wantVars:    map[string]any{"package_name": "redis"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb, err := ParseRaw([]byte(tt.yaml), "test.yaml")
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			task := pb.Plays[0].Tasks[0]
			if task.Include != tt.wantInclude {
				t.Errorf("expected include %q, got %q", tt.wantInclude, task.Include)
			}
			if task.Module != "" {
				t.Errorf("expected empty module for include task, got %q", task.Module)
			}
			if tt.wantWhen != "" && task.When != tt.wantWhen {
				t.Errorf("expected when %q, got %q", tt.wantWhen, task.When)
			}
			if tt.wantVars != nil {
				if task.IncludeVars == nil {
					t.Fatal("expected IncludeVars to be set, got nil")
				}
				for k, v := range tt.wantVars {
					if task.IncludeVars[k] != v {
						t.Errorf("IncludeVars[%q]: expected %v, got %v", k, v, task.IncludeVars[k])
					}
				}
			}
		})
	}
}

func TestIncludeAndIncludeTasksProduceIdenticalTasks(t *testing.T) {
	includeYAML := `
hosts: localhost
tasks:
  - name: Setup
    include: common/setup.yml
    vars:
      pkg: nginx
    when: facts.os == "debian"
`
	includeTasksYAML := `
hosts: localhost
tasks:
  - name: Setup
    include_tasks: common/setup.yml
    vars:
      pkg: nginx
    when: facts.os == "debian"
`
	pb1, err := ParseRaw([]byte(includeYAML), "test.yaml")
	if err != nil {
		t.Fatalf("parse include: %v", err)
	}
	pb2, err := ParseRaw([]byte(includeTasksYAML), "test.yaml")
	if err != nil {
		t.Fatalf("parse include_tasks: %v", err)
	}

	t1 := pb1.Plays[0].Tasks[0]
	t2 := pb2.Plays[0].Tasks[0]

	if t1.Include != t2.Include {
		t.Errorf("Include mismatch: %q vs %q", t1.Include, t2.Include)
	}
	if t1.When != t2.When {
		t.Errorf("When mismatch: %q vs %q", t1.When, t2.When)
	}
	if len(t1.IncludeVars) != len(t2.IncludeVars) {
		t.Errorf("IncludeVars length mismatch: %d vs %d", len(t1.IncludeVars), len(t2.IncludeVars))
	}
	for k, v := range t1.IncludeVars {
		if t2.IncludeVars[k] != v {
			t.Errorf("IncludeVars[%q]: %v vs %v", k, v, t2.IncludeVars[k])
		}
	}
}

func TestParseBlock(t *testing.T) {
	yaml := `
hosts: localhost
tasks:
  - name: Deploy app
    block:
      - name: Pull code
        command:
          cmd: git pull
      - name: Restart service
        command:
          cmd: systemctl restart app
`
	pb, err := ParseRaw([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	task := pb.Plays[0].Tasks[0]
	if !task.IsBlock() {
		t.Fatal("expected task to be a block")
	}
	if task.Module != "" {
		t.Errorf("expected empty module for block task, got %q", task.Module)
	}
	if len(task.Block) != 2 {
		t.Fatalf("expected 2 block tasks, got %d", len(task.Block))
	}
	if task.Block[0].Name != "Pull code" {
		t.Errorf("expected block task 0 name 'Pull code', got %q", task.Block[0].Name)
	}
}

func TestParseBlockWithRescueAlways(t *testing.T) {
	yaml := `
hosts: localhost
tasks:
  - name: Safe deploy
    block:
      - command:
          cmd: deploy.sh
    rescue:
      - command:
          cmd: rollback.sh
    always:
      - command:
          cmd: notify.sh
`
	pb, err := ParseRaw([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	task := pb.Plays[0].Tasks[0]
	if !task.IsBlock() {
		t.Fatal("expected block")
	}
	if len(task.Block) != 1 {
		t.Errorf("expected 1 block task, got %d", len(task.Block))
	}
	if len(task.Rescue) != 1 {
		t.Errorf("expected 1 rescue task, got %d", len(task.Rescue))
	}
	if len(task.Always) != 1 {
		t.Errorf("expected 1 always task, got %d", len(task.Always))
	}
}

func TestParseBlockWithDirectives(t *testing.T) {
	yaml := `
hosts: localhost
tasks:
  - name: Conditional block
    block:
      - command:
          cmd: echo hello
    when: facts.os == "linux"
    sudo: true
`
	pb, err := ParseRaw([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	task := pb.Plays[0].Tasks[0]
	if !task.IsBlock() {
		t.Fatal("expected block")
	}
	if task.When != `facts.os == "linux"` {
		t.Errorf("expected when condition, got %q", task.When)
	}
	if task.Sudo == nil || !*task.Sudo {
		t.Error("expected sudo to be true")
	}
}

func TestParseBlockRejectBlockPlusModule(t *testing.T) {
	yaml := `
hosts: localhost
tasks:
  - name: Bad task
    command:
      cmd: echo hello
    block:
      - command:
          cmd: echo world
`
	_, err := ParseRaw([]byte(yaml), "test.yaml")
	if err == nil {
		t.Fatal("expected error for block + module, got nil")
	}
}

func TestParseBlockRejectRescueWithoutBlock(t *testing.T) {
	yaml := `
hosts: localhost
tasks:
  - name: Bad task
    command:
      cmd: echo hello
    rescue:
      - command:
          cmd: echo fix
`
	_, err := ParseRaw([]byte(yaml), "test.yaml")
	if err == nil {
		t.Fatal("expected error for rescue without block, got nil")
	}
}

func TestParseTagsOnTask(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantTags []string
	}{
		{
			name: "single tag string",
			yaml: `
hosts: localhost
tasks:
  - name: Deploy
    command:
      cmd: deploy.sh
    tags: deploy
`,
			wantTags: []string{"deploy"},
		},
		{
			name: "tag list",
			yaml: `
hosts: localhost
tasks:
  - name: Setup
    command:
      cmd: setup.sh
    tags: [deploy, config]
`,
			wantTags: []string{"deploy", "config"},
		},
		{
			name: "tag list block style",
			yaml: `
hosts: localhost
tasks:
  - name: Setup
    command:
      cmd: setup.sh
    tags:
      - web
      - deploy
`,
			wantTags: []string{"web", "deploy"},
		},
		{
			name: "no tags",
			yaml: `
hosts: localhost
tasks:
  - name: Setup
    command:
      cmd: setup.sh
`,
			wantTags: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb, err := ParseRaw([]byte(tt.yaml), "test.yaml")
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			task := pb.Plays[0].Tasks[0]
			if len(task.Tags) != len(tt.wantTags) {
				t.Fatalf("expected %d tags, got %d: %v", len(tt.wantTags), len(task.Tags), task.Tags)
			}
			for i, tag := range tt.wantTags {
				if task.Tags[i] != tag {
					t.Errorf("tags[%d]: expected %q, got %q", i, tag, task.Tags[i])
				}
			}
		})
	}
}

func TestParseTagsOnPlay(t *testing.T) {
	yaml := `
name: Setup
hosts: localhost
tags: [infra, setup]
tasks:
  - command:
      cmd: echo hello
`
	pb, err := ParseRaw([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	play := pb.Plays[0]
	expected := []string{"infra", "setup"}
	if len(play.Tags) != len(expected) {
		t.Fatalf("expected %d play tags, got %d: %v", len(expected), len(play.Tags), play.Tags)
	}
	for i, tag := range expected {
		if play.Tags[i] != tag {
			t.Errorf("play.Tags[%d]: expected %q, got %q", i, tag, play.Tags[i])
		}
	}
}

func TestParseTagsOnBlock(t *testing.T) {
	yaml := `
hosts: localhost
tasks:
  - name: Deploy block
    tags: [deploy]
    block:
      - name: Pull code
        command:
          cmd: git pull
        tags: config
`
	pb, err := ParseRaw([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	block := pb.Plays[0].Tasks[0]
	if !block.IsBlock() {
		t.Fatal("expected block")
	}
	if len(block.Tags) != 1 || block.Tags[0] != "deploy" {
		t.Errorf("expected block tags [deploy], got %v", block.Tags)
	}
	child := block.Block[0]
	if len(child.Tags) != 1 || child.Tags[0] != "config" {
		t.Errorf("expected child tags [config], got %v", child.Tags)
	}
}

func TestParseTagsOnRoleReference(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantRefs []RoleRef
	}{
		{
			name: "string role (no tags)",
			yaml: `
hosts: localhost
roles:
  - webserver
tasks:
  - command:
      cmd: echo hi
`,
			wantRefs: []RoleRef{{Name: "webserver"}},
		},
		{
			name: "map role with tags",
			yaml: `
hosts: localhost
roles:
  - role: webserver
    tags: [web, deploy]
tasks:
  - command:
      cmd: echo hi
`,
			wantRefs: []RoleRef{{Name: "webserver", Tags: []string{"web", "deploy"}}},
		},
		{
			name: "mixed string and map roles",
			yaml: `
hosts: localhost
roles:
  - common
  - role: webserver
    tags: web
tasks:
  - command:
      cmd: echo hi
`,
			wantRefs: []RoleRef{
				{Name: "common"},
				{Name: "webserver", Tags: []string{"web"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb, err := ParseRaw([]byte(tt.yaml), "test.yaml")
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			play := pb.Plays[0]
			if len(play.Roles) != len(tt.wantRefs) {
				t.Fatalf("expected %d role refs, got %d: %v", len(tt.wantRefs), len(play.Roles), play.Roles)
			}
			for i, want := range tt.wantRefs {
				got := play.Roles[i]
				if got.Name != want.Name {
					t.Errorf("role[%d].Name: expected %q, got %q", i, want.Name, got.Name)
				}
				if len(got.Tags) != len(want.Tags) {
					t.Errorf("role[%d].Tags: expected %v, got %v", i, want.Tags, got.Tags)
					continue
				}
				for j, tag := range want.Tags {
					if got.Tags[j] != tag {
						t.Errorf("role[%d].Tags[%d]: expected %q, got %q", i, j, tag, got.Tags[j])
					}
				}
			}
		})
	}
}

func TestParseAssertTask(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantErr   bool
		errSubstr string
		checkFn   func(t *testing.T, task *Task)
	}{
		{
			name: "assert with single string condition",
			yaml: `
hosts: localhost
tasks:
  - name: check os
    assert:
      that: "facts.os_type == 'Linux'"
`,
			checkFn: func(t *testing.T, task *Task) {
				if task.Assert == nil {
					t.Fatal("expected Assert spec to be set")
				}
				if len(task.Assert.That) != 1 || task.Assert.That[0] != "facts.os_type == 'Linux'" {
					t.Errorf("unexpected That: %+v", task.Assert.That)
				}
				if task.Module != "" {
					t.Errorf("expected empty Module, got %q", task.Module)
				}
			},
		},
		{
			name: "assert with list of conditions and messages",
			yaml: `
hosts: localhost
tasks:
  - assert:
      that:
        - "x == 1"
        - "y == 2"
      fail_msg: "preconditions failed"
      success_msg: "all good"
      quiet: true
`,
			checkFn: func(t *testing.T, task *Task) {
				if task.Assert == nil {
					t.Fatal("expected Assert spec")
				}
				if len(task.Assert.That) != 2 {
					t.Errorf("expected 2 conditions, got %d", len(task.Assert.That))
				}
				if task.Assert.FailMsg != "preconditions failed" {
					t.Errorf("unexpected FailMsg: %q", task.Assert.FailMsg)
				}
				if task.Assert.SuccessMsg != "all good" {
					t.Errorf("unexpected SuccessMsg: %q", task.Assert.SuccessMsg)
				}
				if !task.Assert.Quiet {
					t.Error("expected Quiet=true")
				}
			},
		},
		{
			name: "assert missing that is error",
			yaml: `
hosts: localhost
tasks:
  - assert:
      fail_msg: nope
`,
			wantErr:   true,
			errSubstr: "'that' is required",
		},
		{
			name: "assert empty list is error",
			yaml: `
hosts: localhost
tasks:
  - assert:
      that: []
`,
			wantErr:   true,
			errSubstr: "'that' list is empty",
		},
		{
			name: "assert non-string element is error",
			yaml: `
hosts: localhost
tasks:
  - assert:
      that:
        - "x == 1"
        - 42
`,
			wantErr:   true,
			errSubstr: "is not a string",
		},
		{
			name: "assert cannot coexist with module",
			yaml: `
hosts: localhost
tasks:
  - assert:
      that: "x == 1"
    command:
      cmd: echo
`,
			wantErr:   true,
			errSubstr: "cannot also specify a module",
		},
		{
			name: "assert with register and when",
			yaml: `
hosts: localhost
tasks:
  - name: guarded assert
    when: "run_checks == 'yes'"
    register: my_assert
    assert:
      that:
        - "v is defined"
`,
			checkFn: func(t *testing.T, task *Task) {
				if task.Assert == nil {
					t.Fatal("expected Assert spec")
				}
				if task.Register != "my_assert" {
					t.Errorf("expected Register=my_assert, got %q", task.Register)
				}
				if task.When != "run_checks == 'yes'" {
					t.Errorf("unexpected When: %q", task.When)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb, err := ParseRaw([]byte(tt.yaml), "test.yaml")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
				}
				if tt.errSubstr != "" && !contains(err.Error(), tt.errSubstr) {
					t.Errorf("expected error containing %q, got %q", tt.errSubstr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(pb.Plays) != 1 || len(pb.Plays[0].Tasks) != 1 {
				t.Fatalf("expected 1 play with 1 task")
			}
			if tt.checkFn != nil {
				tt.checkFn(t, pb.Plays[0].Tasks[0])
			}
		})
	}
}

func TestExpandShorthandLeavesAssert(t *testing.T) {
	task := &Task{
		Assert: &AssertSpec{That: []string{"x == 1"}},
		Params: map[string]any{},
	}
	ExpandShorthand(task)
	if task.Assert == nil {
		t.Error("ExpandShorthand should not clear Assert")
	}
	if task.Module != "" {
		t.Errorf("ExpandShorthand should not set Module on assert task, got %q", task.Module)
	}
}

