package cron

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/tackhq/tack/internal/connector"
)

// mockConn is a minimal Connector used for exercising cron.Run / cron.Check.
type mockConn struct {
	// files holds the current content of drop-in files (and the user crontab under key "__crontab__").
	files map[string]string
	// noCrontab, when true, causes crontab -l to exit non-zero with "no crontab for ..."
	noCrontab bool
	// execLog captures every command executed for assertions.
	execLog []string
	// unameOutput controls what `uname -s` returns (defaults to "Linux\n").
	unameOutput string
}

func newMockConn() *mockConn {
	return &mockConn{
		files:       make(map[string]string),
		unameOutput: "Linux\n",
	}
}

const crontabKey = "__crontab__"

func (m *mockConn) Connect(_ context.Context) error { return nil }
func (m *mockConn) Close() error                    { return nil }
func (m *mockConn) String() string                  { return "mock" }
func (m *mockConn) SetSudo(_ bool, _ string)        {}

func (m *mockConn) Execute(_ context.Context, cmd string) (*connector.Result, error) {
	m.execLog = append(m.execLog, cmd)
	switch {
	case cmd == "uname -s":
		return &connector.Result{Stdout: m.unameOutput, ExitCode: 0}, nil
	case strings.HasPrefix(cmd, "crontab"):
		// crontab -l / crontab -u 'X' -l (read)
		if strings.Contains(cmd, " -l") && !strings.Contains(cmd, "<<'") {
			if m.noCrontab {
				return &connector.Result{Stderr: "no crontab for testuser\n", ExitCode: 1}, nil
			}
			return &connector.Result{Stdout: m.files[crontabKey], ExitCode: 0}, nil
		}
		// crontab - <<'...' / crontab -u 'X' - <<'...' (write via heredoc)
		if strings.Contains(cmd, "<<'") {
			newContent := extractHeredoc(cmd)
			m.files[crontabKey] = newContent
			m.noCrontab = false
			return &connector.Result{ExitCode: 0}, nil
		}
		return &connector.Result{ExitCode: 127, Stderr: "unrecognized crontab invocation"}, nil
	case strings.HasPrefix(cmd, "test -e "):
		path := unquote(strings.TrimPrefix(cmd, "test -e "))
		if _, ok := m.files[path]; ok {
			return &connector.Result{ExitCode: 0}, nil
		}
		return &connector.Result{ExitCode: 1}, nil
	case strings.HasPrefix(cmd, "mv "):
		// mv 'tmp' 'dest'
		args := parseTwoQuoted(strings.TrimPrefix(cmd, "mv "))
		if len(args) != 2 {
			return &connector.Result{ExitCode: 1, Stderr: "mv parse error"}, nil
		}
		src, dst := args[0], args[1]
		content, ok := m.files[src]
		if !ok {
			return &connector.Result{ExitCode: 1, Stderr: "no such file"}, nil
		}
		delete(m.files, src)
		m.files[dst] = content
		return &connector.Result{ExitCode: 0}, nil
	case strings.HasPrefix(cmd, "rm -f "):
		path := unquote(strings.TrimPrefix(cmd, "rm -f "))
		delete(m.files, path)
		return &connector.Result{ExitCode: 0}, nil
	}
	return &connector.Result{ExitCode: 127, Stderr: "unknown command: " + cmd}, nil
}

func (m *mockConn) Upload(_ context.Context, src io.Reader, dst string, _ uint32) error {
	b, err := io.ReadAll(src)
	if err != nil {
		return err
	}
	m.files[dst] = string(b)
	return nil
}

func (m *mockConn) Download(_ context.Context, src string, dst io.Writer) error {
	content, ok := m.files[src]
	if !ok {
		return &fsErr{path: src}
	}
	_, err := dst.Write([]byte(content))
	return err
}

type fsErr struct{ path string }

func (e *fsErr) Error() string { return "no such file or directory: " + e.path }

// extractHeredoc pulls the body of a `cmd <<'EOF'\nbody\nEOF` invocation.
func extractHeredoc(cmd string) string {
	// find delimiter between "<<'" and "'\n"
	start := strings.Index(cmd, "<<'")
	if start == -1 {
		return ""
	}
	rest := cmd[start+3:]
	end := strings.IndexByte(rest, '\'')
	if end == -1 {
		return ""
	}
	delim := rest[:end]
	rest = rest[end+1:] // "\nbody\nEOF"
	// drop leading newline
	rest = strings.TrimPrefix(rest, "\n")
	// trim trailing delimiter
	rest = strings.TrimSuffix(rest, delim)
	return rest
}

// unquote strips surrounding single quotes from a shell-quoted argument.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "'")
	s = strings.TrimSuffix(s, "'")
	return s
}

// parseTwoQuoted splits "'a' 'b'" into ["a", "b"].
func parseTwoQuoted(s string) []string {
	var out []string
	for {
		s = strings.TrimSpace(s)
		if s == "" {
			return out
		}
		if !strings.HasPrefix(s, "'") {
			return out
		}
		s = s[1:]
		end := strings.Index(s, "'")
		if end == -1 {
			return out
		}
		out = append(out, s[:end])
		s = s[end+1:]
	}
}

// ----------------------------- Tests ------------------------------------

func TestParseAndValidate_RequiredName(t *testing.T) {
	_, err := parseAndValidate(map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestParseAndValidate_UserWithCronFileAllowed(t *testing.T) {
	// Per design Decision 4: user + cron_file is allowed; user becomes the execution
	// user written into the drop-in line.
	c, err := parseAndValidate(map[string]any{
		"name":      "x",
		"job":       "/bin/true",
		"user":      "alice",
		"cron_file": "/etc/cron.d/x",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if c.user != "alice" {
		t.Errorf("expected user=alice, got %q", c.user)
	}
}

func TestParseAndValidate_SpecialTimeVsFields(t *testing.T) {
	_, err := parseAndValidate(map[string]any{
		"name":         "x",
		"job":          "/bin/true",
		"special_time": "daily",
		"hour":         "3",
	})
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("got %v", err)
	}
}

func TestParseAndValidate_InvalidSpecialTime(t *testing.T) {
	_, err := parseAndValidate(map[string]any{
		"name":         "x",
		"job":          "/bin/true",
		"special_time": "weekly-ish",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid special_time") {
		t.Fatalf("got %v", err)
	}
}

func TestParseAndValidate_EnvModeRequiresKV(t *testing.T) {
	_, err := parseAndValidate(map[string]any{
		"name": "x",
		"env":  true,
		"job":  "no-equals-sign",
	})
	if err == nil || !strings.Contains(err.Error(), "KEY=VALUE") {
		t.Fatalf("got %v", err)
	}
}

func TestParseAndValidate_EnvRejectsSchedule(t *testing.T) {
	_, err := parseAndValidate(map[string]any{
		"name": "x",
		"env":  true,
		"job":  "PATH=/usr/bin",
		"hour": "3",
	})
	if err == nil || !strings.Contains(err.Error(), "env mode") {
		t.Fatalf("got %v", err)
	}
}

func TestParseAndValidate_JobRequiredForPresent(t *testing.T) {
	_, err := parseAndValidate(map[string]any{"name": "x"})
	if err == nil || !strings.Contains(err.Error(), "job is required") {
		t.Fatalf("got %v", err)
	}
}

func TestParseAndValidate_CronFileBasename(t *testing.T) {
	_, err := parseAndValidate(map[string]any{
		"name":      "x",
		"job":       "/bin/true",
		"cron_file": "/etc/cron.d/backup.sh",
	})
	if err == nil || !strings.Contains(err.Error(), "basename") {
		t.Fatalf("got %v", err)
	}
}

func TestParseAndValidate_CronFileAbsPath(t *testing.T) {
	_, err := parseAndValidate(map[string]any{
		"name":      "x",
		"job":       "/bin/true",
		"cron_file": "etc/cron.d/x",
	})
	if err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("got %v", err)
	}
}

func TestParseAndValidate_CronFileDefaultsUserRoot(t *testing.T) {
	c, err := parseAndValidate(map[string]any{
		"name":      "x",
		"job":       "/bin/true",
		"cron_file": "/etc/cron.d/backup",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if c.user != "root" {
		t.Errorf("expected user=root, got %q", c.user)
	}
}

func TestParseAndValidate_StateAbsentAllowsMissingJob(t *testing.T) {
	_, err := parseAndValidate(map[string]any{
		"name":  "x",
		"state": "absent",
	})
	if err != nil {
		t.Errorf("unexpected: %v", err)
	}
}

func TestRun_CreatesInEmptyCrontab(t *testing.T) {
	conn := newMockConn()
	conn.noCrontab = true
	m := &Module{}
	res, err := m.Run(context.Background(), conn, map[string]any{
		"name":   "backup",
		"job":    "/usr/local/bin/backup.sh",
		"hour":   "2",
		"minute": "0",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !res.Changed {
		t.Fatalf("expected changed=true")
	}
	want := "# TACK: backup\n0 2 * * * /usr/local/bin/backup.sh\n"
	if conn.files[crontabKey] != want {
		t.Errorf("crontab content:\n%q\nwant:\n%q", conn.files[crontabKey], want)
	}
}

func TestRun_IdempotentSecondRun(t *testing.T) {
	conn := newMockConn()
	m := &Module{}
	params := map[string]any{
		"name":         "backup",
		"job":          "/bin/b",
		"special_time": "daily",
	}
	if _, err := m.Run(context.Background(), conn, params); err != nil {
		t.Fatalf("first run err: %v", err)
	}
	res, err := m.Run(context.Background(), conn, params)
	if err != nil {
		t.Fatalf("second run err: %v", err)
	}
	if res.Changed {
		t.Fatalf("second run should be unchanged")
	}
}

func TestRun_ScheduleChangeTriggersUpdate(t *testing.T) {
	conn := newMockConn()
	m := &Module{}
	p := map[string]any{"name": "backup", "job": "/bin/b", "hour": "2", "minute": "0"}
	if _, err := m.Run(context.Background(), conn, p); err != nil {
		t.Fatalf("%v", err)
	}
	p["hour"] = "3"
	res, err := m.Run(context.Background(), conn, p)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !res.Changed {
		t.Fatalf("expected changed=true")
	}
	if !strings.Contains(conn.files[crontabKey], "0 3 * * * /bin/b") {
		t.Errorf("updated content not found: %q", conn.files[crontabKey])
	}
}

func TestRun_AbsentRemoves(t *testing.T) {
	conn := newMockConn()
	m := &Module{}
	p := map[string]any{"name": "backup", "job": "/bin/b", "special_time": "daily"}
	if _, err := m.Run(context.Background(), conn, p); err != nil {
		t.Fatalf("%v", err)
	}
	res, err := m.Run(context.Background(), conn, map[string]any{"name": "backup", "state": "absent"})
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !res.Changed {
		t.Fatalf("expected changed=true")
	}
	if strings.Contains(conn.files[crontabKey], "backup") {
		t.Errorf("expected backup removed, got: %q", conn.files[crontabKey])
	}
}

func TestRun_AbsentWhenMissingIsUnchanged(t *testing.T) {
	conn := newMockConn()
	conn.noCrontab = true
	m := &Module{}
	res, err := m.Run(context.Background(), conn, map[string]any{"name": "backup", "state": "absent"})
	if err != nil {
		t.Fatalf("%v", err)
	}
	if res.Changed {
		t.Fatalf("expected changed=false")
	}
}

func TestRun_DisableToggle(t *testing.T) {
	conn := newMockConn()
	m := &Module{}
	p := map[string]any{"name": "backup", "job": "/bin/b", "special_time": "daily"}
	if _, err := m.Run(context.Background(), conn, p); err != nil {
		t.Fatalf("%v", err)
	}
	// Disable it
	p["disabled"] = true
	res, err := m.Run(context.Background(), conn, p)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !res.Changed {
		t.Fatal("expected changed=true on disable")
	}
	if !strings.Contains(conn.files[crontabKey], "# @daily /bin/b") {
		t.Errorf("expected commented line, got: %q", conn.files[crontabKey])
	}
	// Idempotent disable
	res, err = m.Run(context.Background(), conn, p)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if res.Changed {
		t.Fatal("expected changed=false on re-disable")
	}
	// Re-enable
	p["disabled"] = false
	res, err = m.Run(context.Background(), conn, p)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !res.Changed {
		t.Fatal("expected changed=true on enable")
	}
}

func TestRun_EnvMode(t *testing.T) {
	conn := newMockConn()
	conn.noCrontab = true
	m := &Module{}
	res, err := m.Run(context.Background(), conn, map[string]any{
		"name": "path",
		"env":  true,
		"job":  "PATH=/usr/local/bin:/usr/bin",
	})
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !res.Changed {
		t.Fatal("expected changed=true")
	}
	want := "# TACK: path\nPATH=/usr/local/bin:/usr/bin\n"
	if conn.files[crontabKey] != want {
		t.Errorf("got %q want %q", conn.files[crontabKey], want)
	}
}

func TestRun_DropInCreate(t *testing.T) {
	conn := newMockConn()
	m := &Module{}
	res, err := m.Run(context.Background(), conn, map[string]any{
		"name":         "backup",
		"job":          "/bin/b",
		"special_time": "daily",
		"cron_file":    "/etc/cron.d/backup",
	})
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !res.Changed {
		t.Fatal("expected changed=true")
	}
	want := "# TACK: backup\n@daily root /bin/b\n"
	if got := conn.files["/etc/cron.d/backup"]; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestRun_DropInDeleteWhenEmpty(t *testing.T) {
	conn := newMockConn()
	m := &Module{}
	p := map[string]any{
		"name":         "backup",
		"job":          "/bin/b",
		"special_time": "daily",
		"cron_file":    "/etc/cron.d/backup",
	}
	if _, err := m.Run(context.Background(), conn, p); err != nil {
		t.Fatalf("%v", err)
	}
	res, err := m.Run(context.Background(), conn, map[string]any{
		"name":      "backup",
		"state":     "absent",
		"cron_file": "/etc/cron.d/backup",
	})
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !res.Changed {
		t.Fatal("expected changed=true")
	}
	if _, exists := conn.files["/etc/cron.d/backup"]; exists {
		t.Error("expected file to be deleted")
	}
}

func TestRun_DropInWithUser(t *testing.T) {
	conn := newMockConn()
	m := &Module{}
	if _, err := m.Run(context.Background(), conn, map[string]any{
		"name":         "backup",
		"job":          "/bin/b",
		"special_time": "daily",
		"cron_file":    "/etc/cron.d/backup",
		"user":         "alice",
	}); err != nil {
		t.Fatalf("%v", err)
	}
	if !strings.Contains(conn.files["/etc/cron.d/backup"], "@daily alice /bin/b") {
		t.Errorf("user field not in line: %q", conn.files["/etc/cron.d/backup"])
	}
}

func TestRun_NonLinuxRejected(t *testing.T) {
	conn := newMockConn()
	conn.unameOutput = "Darwin\n"
	m := &Module{}
	_, err := m.Run(context.Background(), conn, map[string]any{
		"name": "x", "job": "/bin/true", "special_time": "daily",
	})
	if err == nil || !strings.Contains(err.Error(), "Linux") {
		t.Fatalf("expected Linux-only error, got %v", err)
	}
}

func TestCheck_WouldCreate(t *testing.T) {
	conn := newMockConn()
	conn.noCrontab = true
	m := &Module{}
	res, err := m.Check(context.Background(), conn, map[string]any{
		"name": "backup", "job": "/bin/b", "special_time": "daily",
	})
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !res.WouldChange {
		t.Fatal("expected would-change")
	}
	if res.OldContent != "" || res.NewContent == "" {
		t.Errorf("diff content not populated: old=%q new=%q", res.OldContent, res.NewContent)
	}
	// Ensure Check did NOT write anything.
	for _, cmd := range conn.execLog {
		if strings.HasPrefix(cmd, "crontab -") && !strings.HasSuffix(cmd, "-l") && cmd != "crontab -l" {
			// crontab write uses heredoc with `crontab - <<'`
			if strings.Contains(cmd, "<<'") {
				t.Errorf("check mode wrote crontab: %s", cmd)
			}
		}
	}
}

func TestCheck_NoChange(t *testing.T) {
	conn := newMockConn()
	m := &Module{}
	p := map[string]any{"name": "backup", "job": "/bin/b", "special_time": "daily"}
	if _, err := m.Run(context.Background(), conn, p); err != nil {
		t.Fatalf("%v", err)
	}
	res, err := m.Check(context.Background(), conn, p)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if res.WouldChange {
		t.Fatal("expected no change")
	}
}

func TestRun_TargetUserInvokesDashU(t *testing.T) {
	conn := newMockConn()
	conn.noCrontab = true
	m := &Module{}
	_, err := m.Run(context.Background(), conn, map[string]any{
		"name": "x", "job": "/bin/true", "special_time": "daily", "user": "alice",
	})
	if err != nil {
		t.Fatalf("%v", err)
	}
	foundRead, foundWrite := false, false
	for _, cmd := range conn.execLog {
		if cmd == "crontab -u 'alice' -l" {
			foundRead = true
		}
		if strings.HasPrefix(cmd, "crontab -u 'alice' - <<'") {
			foundWrite = true
		}
	}
	if !foundRead {
		t.Errorf("did not see crontab -u alice -l; execLog=%v", conn.execLog)
	}
	if !foundWrite {
		t.Errorf("did not see crontab -u alice - write; execLog=%v", conn.execLog)
	}
}
