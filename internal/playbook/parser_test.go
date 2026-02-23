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
