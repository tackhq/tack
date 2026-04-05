package generate

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/tackhq/tack/internal/connector"
)

// mockConnector implements connector.Connector for testing.
type mockConnector struct {
	commands  map[string]*connector.Result // cmd -> result
	downloads map[string]string            // path -> content
}

func newMockConnector() *mockConnector {
	return &mockConnector{
		commands:  make(map[string]*connector.Result),
		downloads: make(map[string]string),
	}
}

func (m *mockConnector) Connect(ctx context.Context) error    { return nil }
func (m *mockConnector) Close() error                         { return nil }
func (m *mockConnector) String() string                       { return "mock" }
func (m *mockConnector) SetSudo(enabled bool, password string) {}
func (m *mockConnector) Upload(ctx context.Context, src io.Reader, dst string, mode uint32) error {
	return nil
}

func (m *mockConnector) Execute(ctx context.Context, cmd string) (*connector.Result, error) {
	// Check exact matches first
	if r, ok := m.commands[cmd]; ok {
		return r, nil
	}
	// Check prefix matches for flexible matching
	for pattern, r := range m.commands {
		if strings.Contains(cmd, pattern) {
			return r, nil
		}
	}
	return &connector.Result{ExitCode: 1, Stderr: "command not found"}, nil
}

func (m *mockConnector) Download(ctx context.Context, src string, dst io.Writer) error {
	if content, ok := m.downloads[src]; ok {
		_, err := dst.Write([]byte(content))
		return err
	}
	return fmt.Errorf("file not found: %s", src)
}

func (m *mockConnector) onCmd(cmd string, stdout string, exitCode int) {
	m.commands[cmd] = &connector.Result{Stdout: stdout, ExitCode: exitCode}
}

func TestPackageCollectorApt(t *testing.T) {
	// Suppress warnings during test
	old := WarnWriter
	WarnWriter = io.Discard
	defer func() { WarnWriter = old }()

	mock := newMockConnector()
	mock.onCmd("dpkg-query -W -f='${Status}\\n' neovim 2>/dev/null", "install ok installed\n", 0)
	mock.onCmd("dpkg-query -W -f='${Status}\\n' tmux 2>/dev/null", "install ok installed\n", 0)
	mock.onCmd("dpkg-query -W -f='${Status}\\n' missing 2>/dev/null", "", 1)

	facts := map[string]any{"pkg_manager": "apt"}
	tasks, err := (&PackageCollector{}).Collect(context.Background(), mock, []string{"neovim", "tmux", "missing"}, facts)
	if err != nil {
		t.Fatal(err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task (looped), got %d", len(tasks))
	}
	if tasks[0].Module != "apt" {
		t.Errorf("expected module apt, got %s", tasks[0].Module)
	}
	if len(tasks[0].Loop) != 2 {
		t.Errorf("expected 2 loop items, got %d", len(tasks[0].Loop))
	}
}

func TestPackageCollectorBrew(t *testing.T) {
	old := WarnWriter
	WarnWriter = io.Discard
	defer func() { WarnWriter = old }()

	mock := newMockConnector()
	// 2 formulae
	mock.onCmd("brew list --formula 2>/dev/null | grep -x neovim", "neovim\n", 0)
	mock.onCmd("brew list --formula 2>/dev/null | grep -x tmux", "tmux\n", 0)
	// 2 casks
	mock.onCmd("brew list --formula 2>/dev/null | grep -x cleanshot", "", 1)
	mock.onCmd("brew list --cask 2>/dev/null | grep -x cleanshot", "cleanshot\n", 0)
	mock.onCmd("brew list --formula 2>/dev/null | grep -x maccy", "", 1)
	mock.onCmd("brew list --cask 2>/dev/null | grep -x maccy", "maccy\n", 0)

	facts := map[string]any{"pkg_manager": "brew"}
	tasks, err := (&PackageCollector{}).Collect(context.Background(), mock, []string{"neovim", "tmux", "cleanshot", "maccy"}, facts)
	if err != nil {
		t.Fatal(err)
	}

	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks (formulae loop + casks loop), got %d", len(tasks))
	}
	if tasks[0].Module != "brew" {
		t.Errorf("expected module brew, got %s", tasks[0].Module)
	}
	if len(tasks[0].Loop) != 2 {
		t.Errorf("expected 2 formulae in loop, got %d", len(tasks[0].Loop))
	}
	if len(tasks[1].Loop) != 2 {
		t.Errorf("expected 2 casks in loop, got %d", len(tasks[1].Loop))
	}
	if tasks[1].Params["cask"] != true {
		t.Errorf("expected cask: true on cask task")
	}
}

func TestPackageCollectorDnf(t *testing.T) {
	old := WarnWriter
	WarnWriter = io.Discard
	defer func() { WarnWriter = old }()

	mock := newMockConnector()
	mock.onCmd("rpm -q nginx 2>/dev/null", "nginx-1.24.0-1.fc39.x86_64\n", 0)
	mock.onCmd("rpm -q vim 2>/dev/null", "vim-9.0-1.fc39.x86_64\n", 0)

	facts := map[string]any{"pkg_manager": "dnf"}
	tasks, err := (&PackageCollector{}).Collect(context.Background(), mock, []string{"nginx", "vim"}, facts)
	if err != nil {
		t.Fatal(err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task (looped), got %d", len(tasks))
	}
	if tasks[0].Module != "command" {
		t.Errorf("expected module command, got %s", tasks[0].Module)
	}
	if !strings.Contains(tasks[0].Params["cmd"].(string), "dnf install") {
		t.Errorf("expected dnf install command, got %s", tasks[0].Params["cmd"])
	}
}

func TestFileCollectorRegularFile(t *testing.T) {
	old := WarnWriter
	WarnWriter = io.Discard
	defer func() { WarnWriter = old }()

	mock := newMockConnector()
	// stat output
	mock.onCmd("stat -L -c '%F|%a|%U|%G|%s' /etc/hosts", "regular file|644|root|root|235\n", 0)
	// readlink returns empty (not a symlink)
	mock.onCmd("readlink /etc/hosts", "", 1)
	// file content
	mock.downloads["/etc/hosts"] = "127.0.0.1 localhost\n"

	tasks, err := (&FileCollector{}).Collect(context.Background(), mock, []string{"/etc/hosts"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Module != "copy" {
		t.Errorf("expected module copy, got %s", tasks[0].Module)
	}
	if tasks[0].Params["dest"] != "/etc/hosts" {
		t.Errorf("expected dest /etc/hosts, got %v", tasks[0].Params["dest"])
	}
	if tasks[0].Params["content"] != "127.0.0.1 localhost\n" {
		t.Errorf("unexpected content: %v", tasks[0].Params["content"])
	}
}

func TestFileCollectorDirectory(t *testing.T) {
	old := WarnWriter
	WarnWriter = io.Discard
	defer func() { WarnWriter = old }()

	mock := newMockConnector()
	mock.onCmd("stat -L -c '%F|%a|%U|%G|%s' /etc/nginx", "directory|755|root|root|4096\n", 0)
	mock.onCmd("readlink /etc/nginx", "", 1)

	tasks, err := (&FileCollector{}).Collect(context.Background(), mock, []string{"/etc/nginx"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Module != "file" {
		t.Errorf("expected module file, got %s", tasks[0].Module)
	}
	if tasks[0].Params["state"] != "directory" {
		t.Errorf("expected state directory, got %v", tasks[0].Params["state"])
	}
}

func TestFileCollectorSymlink(t *testing.T) {
	old := WarnWriter
	WarnWriter = io.Discard
	defer func() { WarnWriter = old }()

	mock := newMockConnector()
	mock.onCmd("stat -L -c '%F|%a|%U|%G|%s' /usr/local/bin/vim", "regular file|755|root|root|100\n", 0)
	mock.onCmd("readlink /usr/local/bin/vim", "/usr/bin/vim.basic\n", 0)

	tasks, err := (&FileCollector{}).Collect(context.Background(), mock, []string{"/usr/local/bin/vim"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Module != "file" {
		t.Errorf("expected module file, got %s", tasks[0].Module)
	}
	if tasks[0].Params["state"] != "link" {
		t.Errorf("expected state link, got %v", tasks[0].Params["state"])
	}
	if tasks[0].Params["src"] != "/usr/bin/vim.basic" {
		t.Errorf("expected src /usr/bin/vim.basic, got %v", tasks[0].Params["src"])
	}
}

func TestFileCollectorRecursive(t *testing.T) {
	old := WarnWriter
	WarnWriter = io.Discard
	defer func() { WarnWriter = old }()

	mock := newMockConnector()
	// find expands the glob
	mock.onCmd("find /etc/nginx -maxdepth 5", "/etc/nginx\n/etc/nginx/nginx.conf\n/etc/nginx/conf.d\n", 0)
	// stat for each found path
	mock.onCmd("stat -L -c '%F|%a|%U|%G|%s' /etc/nginx 2>/dev/null", "directory|755|root|root|4096\n", 0)
	mock.onCmd("readlink /etc/nginx 2>/dev/null", "", 1)
	mock.onCmd("stat -L -c '%F|%a|%U|%G|%s' /etc/nginx/nginx.conf 2>/dev/null", "regular file|644|root|root|200\n", 0)
	mock.onCmd("readlink /etc/nginx/nginx.conf 2>/dev/null", "", 1)
	mock.downloads["/etc/nginx/nginx.conf"] = "worker_processes auto;\n"
	mock.onCmd("stat -L -c '%F|%a|%U|%G|%s' /etc/nginx/conf.d 2>/dev/null", "directory|755|root|root|4096\n", 0)
	mock.onCmd("readlink /etc/nginx/conf.d 2>/dev/null", "", 1)

	tasks, err := (&FileCollector{}).Collect(context.Background(), mock, []string{"/etc/nginx/*"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks (dir + file + dir), got %d", len(tasks))
	}
	if tasks[0].Module != "file" {
		t.Errorf("expected first task module file, got %s", tasks[0].Module)
	}
	if tasks[1].Module != "copy" {
		t.Errorf("expected second task module copy, got %s", tasks[1].Module)
	}
	if tasks[1].Params["content"] != "worker_processes auto;\n" {
		t.Errorf("unexpected content: %v", tasks[1].Params["content"])
	}
}

func TestServiceCollector(t *testing.T) {
	old := WarnWriter
	WarnWriter = io.Discard
	defer func() { WarnWriter = old }()

	mock := newMockConnector()
	mock.onCmd("systemctl is-active nginx", "active\n", 0)
	mock.onCmd("systemctl is-enabled nginx", "enabled\n", 0)

	tasks, err := (&ServiceCollector{}).Collect(context.Background(), mock, []string{"nginx"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Module != "systemd" {
		t.Errorf("expected module systemd, got %s", tasks[0].Module)
	}
	if tasks[0].Params["state"] != "started" {
		t.Errorf("expected state started, got %v", tasks[0].Params["state"])
	}
	if tasks[0].Params["enabled"] != true {
		t.Errorf("expected enabled true, got %v", tasks[0].Params["enabled"])
	}
}

func TestUserCollector(t *testing.T) {
	old := WarnWriter
	WarnWriter = io.Discard
	defer func() { WarnWriter = old }()

	mock := newMockConnector()
	mock.onCmd("getent passwd deploy", "deploy:x:1001:1001::/home/deploy:/bin/bash\n", 0)
	mock.onCmd("id -Gn deploy", "deploy sudo docker\n", 0)

	tasks, err := (&UserCollector{}).Collect(context.Background(), mock, []string{"deploy"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Module != "command" {
		t.Errorf("expected module command, got %s", tasks[0].Module)
	}
	cmd := tasks[0].Params["cmd"].(string)
	if !strings.Contains(cmd, "--uid 1001") {
		t.Errorf("expected uid 1001 in command, got %s", cmd)
	}
	if !strings.Contains(cmd, "--groups sudo,docker") {
		t.Errorf("expected groups sudo,docker in command, got %s", cmd)
	}
}

func TestMarshalPlaybook(t *testing.T) {
	tasks := []TaskDef{
		{
			Name:   "Install packages",
			Module: "apt",
			Params: map[string]any{"name": "{{ item }}", "state": "present"},
			Loop:   []string{"neovim", "tmux"},
		},
		{
			Name:   "Configure /etc/hosts",
			Module: "copy",
			Params: map[string]any{"dest": "/etc/hosts", "content": "127.0.0.1 localhost\n"},
		},
	}

	opts := Options{
		Hosts:      []string{"web1"},
		Connection: "ssh",
	}

	var buf bytes.Buffer
	err := WriteOutput(&buf, opts, tasks)
	if err != nil {
		t.Fatal(err)
	}

	yaml := buf.String()

	if !strings.Contains(yaml, "Generated by: tack generate") {
		t.Error("missing header comment")
	}
	if !strings.Contains(yaml, "name: Generated playbook") {
		t.Error("missing playbook name")
	}
	if !strings.Contains(yaml, "connection: ssh") {
		t.Error("missing connection")
	}
	if !strings.Contains(yaml, "web1") {
		t.Error("missing host")
	}
	if !strings.Contains(yaml, "Install packages") {
		t.Error("missing task name")
	}
	if !strings.Contains(yaml, "neovim") {
		t.Error("missing loop item")
	}
}
