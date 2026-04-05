package yum

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/tackhq/tack/internal/connector"
)

// mockConnector implements connector.Connector for testing.
type mockConnector struct {
	commands map[string]*connector.Result
}

func newMockConnector() *mockConnector {
	return &mockConnector{
		commands: make(map[string]*connector.Result),
	}
}

func (m *mockConnector) Connect(ctx context.Context) error      { return nil }
func (m *mockConnector) Close() error                           { return nil }
func (m *mockConnector) String() string                         { return "mock" }
func (m *mockConnector) SetSudo(enabled bool, password string)  {}
func (m *mockConnector) Upload(ctx context.Context, src io.Reader, dst string, mode uint32) error {
	return nil
}
func (m *mockConnector) Download(ctx context.Context, src string, dst io.Writer) error {
	return nil
}

func (m *mockConnector) Execute(ctx context.Context, cmd string) (*connector.Result, error) {
	// Check exact matches first
	if r, ok := m.commands[cmd]; ok {
		return r, nil
	}
	// Check substring matches
	for pattern, r := range m.commands {
		if strings.Contains(cmd, pattern) {
			return r, nil
		}
	}
	return &connector.Result{ExitCode: 1, Stderr: "command not found"}, nil
}

func (m *mockConnector) onCmd(cmd string, stdout string, exitCode int) {
	m.commands[cmd] = &connector.Result{Stdout: stdout, ExitCode: exitCode}
}

func TestName(t *testing.T) {
	mod := &Module{}
	if mod.Name() != "yum" {
		t.Errorf("expected name 'yum', got %q", mod.Name())
	}
}

func TestDetectPackageManager(t *testing.T) {
	ctx := context.Background()

	t.Run("prefers dnf", func(t *testing.T) {
		conn := newMockConnector()
		conn.onCmd("command -v dnf", "/usr/bin/dnf\n", 0)
		conn.onCmd("command -v yum", "/usr/bin/yum\n", 0)

		pkgMgr, err := detectPackageManager(ctx, conn)
		if err != nil {
			t.Fatal(err)
		}
		if pkgMgr != "dnf" {
			t.Errorf("expected dnf, got %s", pkgMgr)
		}
	})

	t.Run("falls back to yum", func(t *testing.T) {
		conn := newMockConnector()
		conn.onCmd("command -v yum", "/usr/bin/yum\n", 0)

		pkgMgr, err := detectPackageManager(ctx, conn)
		if err != nil {
			t.Fatal(err)
		}
		if pkgMgr != "yum" {
			t.Errorf("expected yum, got %s", pkgMgr)
		}
	})

	t.Run("error when neither available", func(t *testing.T) {
		conn := newMockConnector()

		_, err := detectPackageManager(ctx, conn)
		if err == nil {
			t.Fatal("expected error when neither dnf nor yum available")
		}
	})
}

func TestRunInstallPresent(t *testing.T) {
	ctx := context.Background()
	conn := newMockConnector()
	conn.onCmd("command -v dnf", "/usr/bin/dnf\n", 0)
	conn.onCmd("rpm -q 'nginx'", "", 1) // not installed
	conn.onCmd("dnf install -y", "", 0)

	mod := &Module{}
	result, err := mod.Run(ctx, conn, map[string]any{
		"name":  "nginx",
		"state": "present",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true for install")
	}
	if !strings.Contains(result.Message, "installed") {
		t.Errorf("expected message to contain 'installed', got %q", result.Message)
	}
}

func TestRunAlreadyInstalled(t *testing.T) {
	ctx := context.Background()
	conn := newMockConnector()
	conn.onCmd("command -v dnf", "/usr/bin/dnf\n", 0)
	conn.onCmd("rpm -q 'nginx'", "nginx-1.24.0-1.el9.x86_64\n", 0)

	mod := &Module{}
	result, err := mod.Run(ctx, conn, map[string]any{
		"name":  "nginx",
		"state": "present",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Error("expected changed=false for already installed package")
	}
}

func TestRunRemove(t *testing.T) {
	ctx := context.Background()
	conn := newMockConnector()
	conn.onCmd("command -v dnf", "/usr/bin/dnf\n", 0)
	conn.onCmd("rpm -q 'nginx'", "nginx-1.24.0-1.el9.x86_64\n", 0)
	conn.onCmd("dnf remove -y", "", 0)

	mod := &Module{}
	result, err := mod.Run(ctx, conn, map[string]any{
		"name":  "nginx",
		"state": "absent",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true for remove")
	}
}

func TestRunRemoveNotInstalled(t *testing.T) {
	ctx := context.Background()
	conn := newMockConnector()
	conn.onCmd("command -v dnf", "/usr/bin/dnf\n", 0)
	conn.onCmd("rpm -q 'nginx'", "", 1)

	mod := &Module{}
	result, err := mod.Run(ctx, conn, map[string]any{
		"name":  "nginx",
		"state": "absent",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Error("expected changed=false when removing non-installed package")
	}
}

func TestRunLatestWithUpdate(t *testing.T) {
	ctx := context.Background()
	conn := newMockConnector()
	conn.onCmd("command -v dnf", "/usr/bin/dnf\n", 0)
	conn.onCmd("rpm -q 'nginx'", "nginx-1.24.0-1.el9.x86_64\n", 0)
	conn.onCmd("dnf check-update", "nginx.x86_64          1.26.0-1.el9          updates\n", 100)
	conn.onCmd("dnf upgrade -y", "", 0)

	mod := &Module{}
	result, err := mod.Run(ctx, conn, map[string]any{
		"name":  "nginx",
		"state": "latest",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true for upgrade")
	}
	if !strings.Contains(result.Message, "upgraded") {
		t.Errorf("expected message to contain 'upgraded', got %q", result.Message)
	}
}

func TestRunLatestAlreadyCurrent(t *testing.T) {
	ctx := context.Background()
	conn := newMockConnector()
	conn.onCmd("command -v dnf", "/usr/bin/dnf\n", 0)
	conn.onCmd("rpm -q 'nginx'", "nginx-1.26.0-1.el9.x86_64\n", 0)
	conn.onCmd("dnf check-update", "", 0) // no updates

	mod := &Module{}
	result, err := mod.Run(ctx, conn, map[string]any{
		"name":  "nginx",
		"state": "latest",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Error("expected changed=false when already at latest")
	}
}

func TestRunUpdateCache(t *testing.T) {
	ctx := context.Background()
	conn := newMockConnector()
	conn.onCmd("command -v dnf", "/usr/bin/dnf\n", 0)
	conn.onCmd("dnf makecache", "", 0)

	mod := &Module{}
	result, err := mod.Run(ctx, conn, map[string]any{
		"update_cache": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true for cache update")
	}
}

func TestRunUpgradeAll(t *testing.T) {
	ctx := context.Background()
	conn := newMockConnector()
	conn.onCmd("command -v dnf", "/usr/bin/dnf\n", 0)
	conn.onCmd("dnf upgrade -y", "Upgraded: 3 packages\n", 0)

	mod := &Module{}
	result, err := mod.Run(ctx, conn, map[string]any{
		"upgrade": "yes",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true for upgrade all")
	}
}

func TestRunAutoremove(t *testing.T) {
	ctx := context.Background()
	conn := newMockConnector()
	conn.onCmd("command -v dnf", "/usr/bin/dnf\n", 0)
	conn.onCmd("rpm -q 'nginx'", "nginx-1.24.0-1.el9.x86_64\n", 0)
	conn.onCmd("dnf remove -y", "", 0)
	conn.onCmd("dnf autoremove -y", "Removed: libfoo-1.0\n", 0)

	mod := &Module{}
	result, err := mod.Run(ctx, conn, map[string]any{
		"name":       "nginx",
		"state":      "absent",
		"autoremove": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true")
	}
	if !strings.Contains(result.Message, "autoremove") {
		t.Errorf("expected message to contain 'autoremove', got %q", result.Message)
	}
}

func TestRunInvalidState(t *testing.T) {
	ctx := context.Background()
	conn := newMockConnector()
	conn.onCmd("command -v dnf", "/usr/bin/dnf\n", 0)

	mod := &Module{}
	_, err := mod.Run(ctx, conn, map[string]any{
		"name":  "nginx",
		"state": "invalid",
	})
	if err == nil {
		t.Fatal("expected error for invalid state")
	}
}

func TestRunNoNameNoAction(t *testing.T) {
	ctx := context.Background()
	conn := newMockConnector()
	conn.onCmd("command -v dnf", "/usr/bin/dnf\n", 0)

	mod := &Module{}
	_, err := mod.Run(ctx, conn, map[string]any{})
	if err == nil {
		t.Fatal("expected error when no name and no action specified")
	}
}

func TestCheckWouldInstall(t *testing.T) {
	ctx := context.Background()
	conn := newMockConnector()
	conn.onCmd("command -v dnf", "/usr/bin/dnf\n", 0)
	conn.onCmd("rpm -q 'nginx'", "", 1) // not installed

	mod := &Module{}
	result, err := mod.Check(ctx, conn, map[string]any{
		"name":  "nginx",
		"state": "present",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.WouldChange {
		t.Error("expected WouldChange=true")
	}
}

func TestCheckNoChange(t *testing.T) {
	ctx := context.Background()
	conn := newMockConnector()
	conn.onCmd("command -v dnf", "/usr/bin/dnf\n", 0)
	conn.onCmd("rpm -q 'nginx'", "nginx-1.24.0-1.el9.x86_64\n", 0)

	mod := &Module{}
	result, err := mod.Check(ctx, conn, map[string]any{
		"name":  "nginx",
		"state": "present",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.WouldChange {
		t.Error("expected WouldChange=false")
	}
}

func TestCheckUpgradeUncertain(t *testing.T) {
	ctx := context.Background()
	conn := newMockConnector()
	conn.onCmd("command -v dnf", "/usr/bin/dnf\n", 0)

	mod := &Module{}
	result, err := mod.Check(ctx, conn, map[string]any{
		"upgrade": "yes",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Uncertain {
		t.Error("expected Uncertain=true for upgrade")
	}
}

func TestMultiplePackages(t *testing.T) {
	ctx := context.Background()
	conn := newMockConnector()
	conn.onCmd("command -v dnf", "/usr/bin/dnf\n", 0)
	conn.onCmd("rpm -q 'nginx'", "nginx-1.24.0-1.el9.x86_64\n", 0) // installed
	conn.onCmd("rpm -q 'curl'", "", 1)                               // not installed
	conn.onCmd("rpm -q 'wget'", "", 1)                               // not installed
	conn.onCmd("dnf install -y", "", 0)

	mod := &Module{}
	result, err := mod.Run(ctx, conn, map[string]any{
		"name":  []any{"nginx", "curl", "wget"},
		"state": "present",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true")
	}
	if !strings.Contains(result.Message, "curl") || !strings.Contains(result.Message, "wget") {
		t.Errorf("expected message to list curl and wget, got %q", result.Message)
	}
	if strings.Contains(result.Message, "nginx") {
		t.Errorf("nginx should not appear in install message since already installed, got %q", result.Message)
	}
}

func TestYumFallback(t *testing.T) {
	ctx := context.Background()
	conn := newMockConnector()
	// dnf not available, yum available
	conn.onCmd("command -v yum", "/usr/bin/yum\n", 0)
	conn.onCmd("rpm -q 'nginx'", "", 1)
	conn.onCmd("yum install -y", "", 0)

	mod := &Module{}
	result, err := mod.Run(ctx, conn, map[string]any{
		"name":  "nginx",
		"state": "present",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true")
	}
}
