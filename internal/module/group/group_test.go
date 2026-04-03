package group

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/eugenetaranov/bolt/internal/connector"
)

type mockConnector struct {
	commands map[string]*connector.Result
	executed []string
}

func newMockConnector() *mockConnector {
	return &mockConnector{
		commands: make(map[string]*connector.Result),
	}
}

func (m *mockConnector) Connect(ctx context.Context) error     { return nil }
func (m *mockConnector) Close() error                          { return nil }
func (m *mockConnector) String() string                        { return "mock" }
func (m *mockConnector) SetSudo(enabled bool, password string) {}
func (m *mockConnector) Upload(ctx context.Context, src io.Reader, dst string, mode uint32) error {
	return nil
}
func (m *mockConnector) Download(ctx context.Context, src string, dst io.Writer) error {
	return nil
}

func (m *mockConnector) Execute(ctx context.Context, cmd string) (*connector.Result, error) {
	m.executed = append(m.executed, cmd)
	if r, ok := m.commands[cmd]; ok {
		return r, nil
	}
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
	if mod.Name() != "group" {
		t.Errorf("expected 'group', got %q", mod.Name())
	}
}

func TestGetGroupInfo(t *testing.T) {
	ctx := context.Background()

	t.Run("group exists", func(t *testing.T) {
		conn := newMockConnector()
		conn.onCmd("getent group 'deploy'", "deploy:x:1500:\n", 0)

		info, err := getGroupInfo(ctx, conn, "deploy")
		if err != nil {
			t.Fatal(err)
		}
		if !info.Exists {
			t.Error("expected group to exist")
		}
		if info.GID != 1500 {
			t.Errorf("expected gid 1500, got %d", info.GID)
		}
	})

	t.Run("group does not exist", func(t *testing.T) {
		conn := newMockConnector()
		conn.onCmd("getent group 'nogroup'", "", 2)

		info, err := getGroupInfo(ctx, conn, "nogroup")
		if err != nil {
			t.Fatal(err)
		}
		if info.Exists {
			t.Error("expected group to not exist")
		}
	})
}

func TestRunCreateGroup(t *testing.T) {
	ctx := context.Background()
	mod := &Module{}

	t.Run("create with defaults", func(t *testing.T) {
		conn := newMockConnector()
		conn.onCmd("getent group 'deploy'", "", 2)
		conn.onCmd("groupadd 'deploy'", "", 0)

		result, err := mod.Run(ctx, conn, map[string]any{"name": "deploy"})
		if err != nil {
			t.Fatal(err)
		}
		if !result.Changed {
			t.Error("expected changed")
		}
	})

	t.Run("create with gid", func(t *testing.T) {
		conn := newMockConnector()
		conn.onCmd("getent group 'deploy'", "", 2)
		conn.onCmd("groupadd -g 1500 'deploy'", "", 0)

		result, err := mod.Run(ctx, conn, map[string]any{"name": "deploy", "gid": 1500})
		if err != nil {
			t.Fatal(err)
		}
		if !result.Changed {
			t.Error("expected changed")
		}
	})

	t.Run("create system group", func(t *testing.T) {
		conn := newMockConnector()
		conn.onCmd("getent group 'deploy'", "", 2)
		conn.onCmd("groupadd -r 'deploy'", "", 0)

		result, err := mod.Run(ctx, conn, map[string]any{"name": "deploy", "system": true})
		if err != nil {
			t.Fatal(err)
		}
		if !result.Changed {
			t.Error("expected changed")
		}
	})
}

func TestRunGroupIdempotent(t *testing.T) {
	ctx := context.Background()
	mod := &Module{}

	t.Run("group already exists with matching gid", func(t *testing.T) {
		conn := newMockConnector()
		conn.onCmd("getent group 'deploy'", "deploy:x:1500:\n", 0)

		result, err := mod.Run(ctx, conn, map[string]any{"name": "deploy", "gid": 1500})
		if err != nil {
			t.Fatal(err)
		}
		if result.Changed {
			t.Error("expected no change")
		}
	})

	t.Run("group exists no gid specified", func(t *testing.T) {
		conn := newMockConnector()
		conn.onCmd("getent group 'deploy'", "deploy:x:1000:\n", 0)

		result, err := mod.Run(ctx, conn, map[string]any{"name": "deploy"})
		if err != nil {
			t.Fatal(err)
		}
		if result.Changed {
			t.Error("expected no change")
		}
	})
}

func TestRunGroupModify(t *testing.T) {
	ctx := context.Background()
	mod := &Module{}

	t.Run("change gid", func(t *testing.T) {
		conn := newMockConnector()
		conn.onCmd("getent group 'deploy'", "deploy:x:1500:\n", 0)
		conn.onCmd("groupmod -g 1600 'deploy'", "", 0)

		result, err := mod.Run(ctx, conn, map[string]any{"name": "deploy", "gid": 1600})
		if err != nil {
			t.Fatal(err)
		}
		if !result.Changed {
			t.Error("expected changed")
		}
	})
}

func TestRunGroupRemove(t *testing.T) {
	ctx := context.Background()
	mod := &Module{}

	t.Run("remove existing group", func(t *testing.T) {
		conn := newMockConnector()
		conn.onCmd("getent group 'deploy'", "deploy:x:1500:\n", 0)
		conn.onCmd("groupdel 'deploy'", "", 0)

		result, err := mod.Run(ctx, conn, map[string]any{"name": "deploy", "state": "absent"})
		if err != nil {
			t.Fatal(err)
		}
		if !result.Changed {
			t.Error("expected changed")
		}
	})

	t.Run("remove non-existent group", func(t *testing.T) {
		conn := newMockConnector()
		conn.onCmd("getent group 'deploy'", "", 2)

		result, err := mod.Run(ctx, conn, map[string]any{"name": "deploy", "state": "absent"})
		if err != nil {
			t.Fatal(err)
		}
		if result.Changed {
			t.Error("expected no change")
		}
	})
}

func TestRunGroupValidation(t *testing.T) {
	ctx := context.Background()
	mod := &Module{}

	t.Run("missing name", func(t *testing.T) {
		conn := newMockConnector()
		_, err := mod.Run(ctx, conn, map[string]any{})
		if err == nil {
			t.Error("expected error for missing name")
		}
	})

	t.Run("invalid state", func(t *testing.T) {
		conn := newMockConnector()
		_, err := mod.Run(ctx, conn, map[string]any{"name": "deploy", "state": "invalid"})
		if err == nil {
			t.Error("expected error for invalid state")
		}
	})
}

func TestCheckGroup(t *testing.T) {
	ctx := context.Background()
	mod := &Module{}

	t.Run("would create", func(t *testing.T) {
		conn := newMockConnector()
		conn.onCmd("getent group 'deploy'", "", 2)

		cr, err := mod.Check(ctx, conn, map[string]any{"name": "deploy"})
		if err != nil {
			t.Fatal(err)
		}
		if !cr.WouldChange {
			t.Error("expected would change")
		}
	})

	t.Run("no change needed", func(t *testing.T) {
		conn := newMockConnector()
		conn.onCmd("getent group 'deploy'", "deploy:x:1500:\n", 0)

		cr, err := mod.Check(ctx, conn, map[string]any{"name": "deploy", "gid": 1500})
		if err != nil {
			t.Fatal(err)
		}
		if cr.WouldChange {
			t.Error("expected no change")
		}
	})

	t.Run("would remove", func(t *testing.T) {
		conn := newMockConnector()
		conn.onCmd("getent group 'deploy'", "deploy:x:1500:\n", 0)

		cr, err := mod.Check(ctx, conn, map[string]any{"name": "deploy", "state": "absent"})
		if err != nil {
			t.Fatal(err)
		}
		if !cr.WouldChange {
			t.Error("expected would change")
		}
	})
}
