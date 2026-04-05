package git

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/tackhq/tack/internal/connector"
)

// ---------- Mock connector ----------

type responder func(cmd string) (*connector.Result, error)

type mockConn struct {
	execLog     []string
	responders  []responder
	defaultResp func(cmd string) (*connector.Result, error)
}

func newMockConn() *mockConn {
	return &mockConn{
		defaultResp: func(cmd string) (*connector.Result, error) {
			return &connector.Result{ExitCode: 127, Stderr: "unknown: " + cmd}, nil
		},
	}
}

// when(prefix, result) adds a matcher that matches commands that contain `contains`.
func (m *mockConn) when(contains string, stdout string, exitCode int) *mockConn {
	m.responders = append(m.responders, func(cmd string) (*connector.Result, error) {
		if strings.Contains(cmd, contains) {
			return &connector.Result{Stdout: stdout, ExitCode: exitCode}, nil
		}
		return nil, nil
	})
	return m
}

func (m *mockConn) Connect(_ context.Context) error { return nil }
func (m *mockConn) Close() error                    { return nil }
func (m *mockConn) String() string                  { return "mock" }
func (m *mockConn) SetSudo(_ bool, _ string)        {}

func (m *mockConn) Execute(_ context.Context, cmd string) (*connector.Result, error) {
	m.execLog = append(m.execLog, cmd)
	for _, r := range m.responders {
		res, err := r(cmd)
		if err != nil {
			return nil, err
		}
		if res != nil {
			return res, nil
		}
	}
	return m.defaultResp(cmd)
}

func (m *mockConn) Upload(_ context.Context, _ io.Reader, _ string, _ uint32) error { return nil }
func (m *mockConn) Download(_ context.Context, _ string, _ io.Writer) error         { return nil }

// logContains reports whether any logged command contains substr.
func (m *mockConn) logContains(substr string) bool {
	for _, c := range m.execLog {
		if strings.Contains(c, substr) {
			return true
		}
	}
	return false
}

// ---------- Validation tests ----------

func TestParseAndValidate_MissingRepo(t *testing.T) {
	_, err := parseAndValidate(map[string]any{"dest": "/opt/app"})
	if err == nil || !strings.Contains(err.Error(), "repo") {
		t.Fatalf("expected missing repo error, got %v", err)
	}
}

func TestParseAndValidate_MissingDest(t *testing.T) {
	_, err := parseAndValidate(map[string]any{"repo": "git@github.com:x/y.git"})
	if err == nil || !strings.Contains(err.Error(), "dest") {
		t.Fatalf("expected missing dest error, got %v", err)
	}
}

func TestParseAndValidate_RelativeDest(t *testing.T) {
	_, err := parseAndValidate(map[string]any{"repo": "r", "dest": "./repo"})
	if err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("expected absolute-path error, got %v", err)
	}
}

func TestParseAndValidate_NegativeDepth(t *testing.T) {
	_, err := parseAndValidate(map[string]any{"repo": "r", "dest": "/opt/a", "depth": -1})
	if err == nil || !strings.Contains(err.Error(), "depth") {
		t.Fatalf("expected depth error, got %v", err)
	}
}

func TestParseAndValidate_EmptyVersion(t *testing.T) {
	_, err := parseAndValidate(map[string]any{"repo": "r", "dest": "/opt/a", "version": "   "})
	if err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("expected version error, got %v", err)
	}
}

func TestParseAndValidate_SHADetection(t *testing.T) {
	cases := []struct {
		version string
		isSHA   bool
	}{
		{"abcdef1", true},
		{"abcdef1234567890abcdef1234567890abcdef12", true},
		{"main", false},
		{"v1.2.3", false},
		{"abcdef", false}, // too short
		{"abcdefg", false}, // non-hex char
	}
	for _, tc := range cases {
		c, err := parseAndValidate(map[string]any{"repo": "r", "dest": "/opt/a", "version": tc.version})
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if c.versionIsSHA != tc.isSHA {
			t.Errorf("version %q: expected isSHA=%v, got %v", tc.version, tc.isSHA, c.versionIsSHA)
		}
	}
}

func TestParseAndValidate_Defaults(t *testing.T) {
	c, err := parseAndValidate(map[string]any{"repo": "r", "dest": "/opt/a"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !c.update || !c.clone {
		t.Errorf("expected update and clone defaults true")
	}
	if c.force || c.bare || c.singleBranch || c.recursive || c.acceptHostKey {
		t.Errorf("expected false defaults for bools")
	}
}

// ---------- sshCommand tests ----------

func TestSSHCommand(t *testing.T) {
	c := &config{}
	if c.sshCommand() != "" {
		t.Error("empty config should produce empty sshCommand")
	}
	c = &config{acceptHostKey: true}
	if !strings.Contains(c.sshCommand(), "accept-new") {
		t.Errorf("accept_hostkey not applied: %q", c.sshCommand())
	}
	c = &config{keyFile: "/home/x/.ssh/id"}
	got := c.sshCommand()
	if !strings.Contains(got, "-i /home/x/.ssh/id") || !strings.Contains(got, "IdentitiesOnly=yes") {
		t.Errorf("key_file not applied: %q", got)
	}
	c = &config{keyFile: "/k", acceptHostKey: true}
	got = c.sshCommand()
	if !strings.Contains(got, "accept-new") || !strings.Contains(got, "-i /k") {
		t.Errorf("combined flags missing: %q", got)
	}
}

// ---------- resolveVersion tests ----------

func TestResolveVersion_SHA(t *testing.T) {
	conn := newMockConn()
	sha := "abcdef1234567890abcdef1234567890abcdef12"
	got, _, err := resolveVersion(context.Background(), conn, "r", sha, "")
	if err != nil || got != sha {
		t.Fatalf("sha pass-through failed: got=%q err=%v", got, err)
	}
	if len(conn.execLog) != 0 {
		t.Error("SHA pass-through should not issue any commands")
	}
}

func TestResolveVersion_Branch(t *testing.T) {
	conn := newMockConn()
	conn.when("ls-remote", "aabbcc1122334455667788990011223344556677\trefs/heads/main\n", 0)
	got, ref, err := resolveVersion(context.Background(), conn, "r", "main", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "aabbcc1122334455667788990011223344556677" {
		t.Errorf("bad sha: %q", got)
	}
	if ref != "refs/heads/main" {
		t.Errorf("bad ref: %q", ref)
	}
}

func TestResolveVersion_Tag(t *testing.T) {
	conn := newMockConn()
	// Annotated tags: first line is the tag object, second is the peeled commit (^{}).
	out := "1111111111111111111111111111111111111111\trefs/tags/v1.0\n" +
		"2222222222222222222222222222222222222222\trefs/tags/v1.0^{}\n"
	conn.when("ls-remote", out, 0)
	got, ref, err := resolveVersion(context.Background(), conn, "r", "v1.0", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "2222222222222222222222222222222222222222" {
		t.Errorf("expected peeled tag SHA, got %q", got)
	}
	if ref != "refs/tags/v1.0" {
		t.Errorf("bad ref: %q", ref)
	}
}

func TestResolveVersion_DefaultHEAD(t *testing.T) {
	conn := newMockConn()
	out := "ref: refs/heads/main\tHEAD\n" +
		"3333333333333333333333333333333333333333\tHEAD\n"
	conn.when("ls-remote --symref", out, 0)
	got, ref, err := resolveVersion(context.Background(), conn, "r", "", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "3333333333333333333333333333333333333333" {
		t.Errorf("bad sha: %q", got)
	}
	if ref != "refs/heads/main" {
		t.Errorf("bad ref: %q", ref)
	}
}

func TestResolveVersion_Unknown(t *testing.T) {
	conn := newMockConn()
	conn.when("ls-remote", "", 0) // empty output
	_, _, err := resolveVersion(context.Background(), conn, "r", "nonexistent", "")
	if err == nil || !strings.Contains(err.Error(), "could not resolve") {
		t.Fatalf("expected unresolved error, got %v", err)
	}
}

func TestResolveVersion_UsesSSHCommand(t *testing.T) {
	conn := newMockConn()
	conn.when("ls-remote", "aabbcc1122334455667788990011223344556677\trefs/heads/main\n", 0)
	_, _, err := resolveVersion(context.Background(), conn, "r", "main", "ssh -i /k")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !conn.logContains("GIT_SSH_COMMAND='ssh -i /k'") {
		t.Errorf("expected GIT_SSH_COMMAND in executed cmd; log=%v", conn.execLog)
	}
}

// ---------- Inspection helpers ----------

func TestIsGitRepo(t *testing.T) {
	conn := newMockConn()
	conn.when("test -e", "", 0)
	ok, err := isGitRepo(context.Background(), conn, "/opt/a")
	if err != nil || !ok {
		t.Fatalf("expected true, got ok=%v err=%v", ok, err)
	}
	conn = newMockConn()
	conn.when("test -e", "", 1)
	ok, err = isGitRepo(context.Background(), conn, "/opt/a")
	if err != nil || ok {
		t.Fatalf("expected false, got ok=%v err=%v", ok, err)
	}
}

func TestCurrentSHA(t *testing.T) {
	conn := newMockConn()
	conn.when("rev-parse HEAD", "aabbccdd\n", 0)
	got, err := currentSHA(context.Background(), conn, "/opt/a")
	if err != nil || got != "aabbccdd" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestIsDirty(t *testing.T) {
	conn := newMockConn()
	conn.when("status --porcelain", "", 0)
	dirty, _, _ := isDirty(context.Background(), conn, "/opt/a")
	if dirty {
		t.Error("expected clean")
	}
	conn = newMockConn()
	conn.when("status --porcelain", " M foo\n", 0)
	dirty, paths, _ := isDirty(context.Background(), conn, "/opt/a")
	if !dirty || !strings.Contains(paths, "foo") {
		t.Errorf("expected dirty with foo; got dirty=%v paths=%q", dirty, paths)
	}
}

// ---------- Full Run tests (with mock connector) ----------

// newRepoMock builds a mock covering the common git commands. The caller wires
// in specific responses afterward via when/whenErr.
func newRepoMock(existing bool, headSHA string, resolved string, dirty bool) *mockConn {
	conn := newMockConn()
	// git binary precheck
	conn.when("command -v git", "/usr/bin/git", 0)
	// test -e for isGitRepo
	if existing {
		conn.when("test -e", "", 0)
	} else {
		conn.when("test -e", "", 1)
	}
	// rev-parse HEAD
	conn.when("rev-parse HEAD", headSHA+"\n", 0)
	// ls-remote
	conn.when("ls-remote", resolved+"\trefs/heads/main\n", 0)
	// status --porcelain
	if dirty {
		conn.when("status --porcelain", " M foo\n", 0)
	} else {
		conn.when("status --porcelain", "", 0)
	}
	// remote get-url
	conn.when("remote get-url origin", "git@github.com:x/y.git\n", 0)
	// mkdir, clone, fetch, checkout, reset, clean, submodule — all succeed quietly
	for _, tok := range []string{"mkdir -p", "git clone", "fetch", "checkout", "reset", "clean", "submodule"} {
		conn.when(tok, "", 0)
	}
	return conn
}

func TestRun_FreshClone(t *testing.T) {
	conn := newRepoMock(false, "", "abcabcabcabcabcabcabcabcabcabcabcabcabca", false)
	m := &Module{}
	res, err := m.Run(context.Background(), conn, map[string]any{
		"repo": "git@github.com:x/y.git",
		"dest": "/opt/app",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !res.Changed {
		t.Error("expected changed=true on fresh clone")
	}
	if !conn.logContains("git clone") {
		t.Error("expected git clone to be invoked")
	}
	if res.Data["before_sha"] != "" {
		t.Errorf("before_sha should be empty, got %v", res.Data["before_sha"])
	}
}

func TestRun_IdempotentNoOp(t *testing.T) {
	sha := "abcabcabcabcabcabcabcabcabcabcabcabcabca"
	conn := newRepoMock(true, sha, sha, false)
	m := &Module{}
	res, err := m.Run(context.Background(), conn, map[string]any{
		"repo":    "r",
		"dest":    "/opt/app",
		"version": "main",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Changed {
		t.Error("expected changed=false when SHA matches")
	}
	if conn.logContains("git clone") || conn.logContains("fetch") || conn.logContains("checkout") {
		t.Errorf("no git clone/fetch/checkout should occur; log=%v", conn.execLog)
	}
}

func TestRun_UpdateWithSHAChange(t *testing.T) {
	oldSHA := "1111111111111111111111111111111111111111"
	newSHA := "2222222222222222222222222222222222222222"
	conn := newRepoMock(true, oldSHA, newSHA, false)
	m := &Module{}
	res, err := m.Run(context.Background(), conn, map[string]any{
		"repo":    "r",
		"dest":    "/opt/app",
		"version": "main",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !res.Changed {
		t.Error("expected changed=true")
	}
	if !conn.logContains("fetch") || !conn.logContains("checkout") {
		t.Errorf("expected fetch+checkout; log=%v", conn.execLog)
	}
}

func TestRun_UpdateFalseSkips(t *testing.T) {
	sha := "1111111111111111111111111111111111111111"
	conn := newRepoMock(true, sha, "2222222222222222222222222222222222222222", false)
	m := &Module{}
	res, err := m.Run(context.Background(), conn, map[string]any{
		"repo":   "r",
		"dest":   "/opt/app",
		"update": false,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Changed {
		t.Error("expected changed=false with update=false")
	}
	if conn.logContains("ls-remote") || conn.logContains("fetch") {
		t.Errorf("update=false should skip ls-remote/fetch; log=%v", conn.execLog)
	}
}

func TestRun_CloneFalseFails(t *testing.T) {
	conn := newRepoMock(false, "", "abcabcabcabcabcabcabcabcabcabcabcabcabca", false)
	m := &Module{}
	_, err := m.Run(context.Background(), conn, map[string]any{
		"repo":  "r",
		"dest":  "/opt/app",
		"clone": false,
	})
	if err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("expected clone=false error, got %v", err)
	}
}

func TestRun_DirtyFails(t *testing.T) {
	oldSHA := "1111111111111111111111111111111111111111"
	newSHA := "2222222222222222222222222222222222222222"
	conn := newRepoMock(true, oldSHA, newSHA, true)
	m := &Module{}
	_, err := m.Run(context.Background(), conn, map[string]any{
		"repo": "r", "dest": "/opt/app", "version": "main",
	})
	if err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("expected dirty error, got %v", err)
	}
}

func TestRun_DirtyForceResets(t *testing.T) {
	oldSHA := "1111111111111111111111111111111111111111"
	newSHA := "2222222222222222222222222222222222222222"
	conn := newRepoMock(true, oldSHA, newSHA, true)
	m := &Module{}
	res, err := m.Run(context.Background(), conn, map[string]any{
		"repo": "r", "dest": "/opt/app", "version": "main", "force": true,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !res.Changed {
		t.Error("expected changed=true")
	}
	if !conn.logContains("reset --hard") || !conn.logContains("clean -fdx") {
		t.Errorf("expected reset+clean with force=true; log=%v", conn.execLog)
	}
}

func TestRun_ForceCleanWorktreeIsNoOp(t *testing.T) {
	// force=true on a clean worktree at desired ref should still be a no-op —
	// the idempotency guard fires before we ever consider force.
	sha := "abcabcabcabcabcabcabcabcabcabcabcabcabca"
	conn := newRepoMock(true, sha, sha, false)
	m := &Module{}
	res, err := m.Run(context.Background(), conn, map[string]any{
		"repo": "r", "dest": "/opt/app", "version": "main", "force": true,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Changed {
		t.Error("expected no change")
	}
	if conn.logContains("reset --hard") || conn.logContains("clean -fdx") {
		t.Errorf("force should NOT redundantly reset a clean worktree; log=%v", conn.execLog)
	}
}

func TestRun_BareClone(t *testing.T) {
	conn := newRepoMock(false, "", "abcabcabcabcabcabcabcabcabcabcabcabcabca", false)
	m := &Module{}
	res, err := m.Run(context.Background(), conn, map[string]any{
		"repo": "r", "dest": "/opt/app.git", "bare": true,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !res.Changed {
		t.Error("expected changed=true")
	}
	// Must use --bare flag
	found := false
	for _, c := range conn.execLog {
		if strings.Contains(c, "git clone") && strings.Contains(c, "--bare") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected --bare flag; log=%v", conn.execLog)
	}
	// Must NOT run status --porcelain or checkout
	if conn.logContains("status --porcelain") || conn.logContains("checkout") {
		t.Errorf("bare clone should skip dirty/checkout; log=%v", conn.execLog)
	}
}

func TestRun_ShallowSHAFallback(t *testing.T) {
	// Existing shallow repo at oldSHA; targeting a new SHA at depth=1.
	// First fetch of the specific SHA fails; fallback to --unshallow succeeds.
	oldSHA := "1111111111111111111111111111111111111111"
	newSHA := "2222222222222222222222222222222222222222"
	conn := newMockConn()
	conn.when("command -v git", "/usr/bin/git", 0)
	conn.when("test -e", "", 0)
	conn.when("rev-parse HEAD", oldSHA+"\n", 0)
	// No ls-remote needed because version is a SHA.
	conn.when("status --porcelain", "", 0)
	conn.when("remote get-url origin", "r\n", 0)
	// First fetch attempt (specific SHA) — fail.
	conn.when("fetch --depth=1 origin "+newSHA, "access denied", 1)
	// Unshallow fetch — succeed.
	conn.when("fetch --unshallow origin", "", 0)
	conn.when("checkout", "", 0)
	conn.when("rev-parse HEAD", newSHA+"\n", 0) // after checkout

	m := &Module{}
	res, err := m.Run(context.Background(), conn, map[string]any{
		"repo": "r", "dest": "/opt/app", "version": newSHA, "depth": 1,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	warnings, _ := res.Data["warnings"].([]string)
	if len(warnings) == 0 {
		t.Errorf("expected a warning about shallow SHA fallback; data=%v", res.Data)
	}
}

// ---------- Check (dry-run) tests ----------

func TestCheck_WouldClone(t *testing.T) {
	conn := newRepoMock(false, "", "abcabcabcabcabcabcabcabcabcabcabcabcabca", false)
	m := &Module{}
	cr, err := m.Check(context.Background(), conn, map[string]any{
		"repo": "r", "dest": "/opt/app",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !cr.WouldChange {
		t.Error("expected WouldChange=true")
	}
	if cr.OldChecksum != "" || cr.NewChecksum == "" {
		t.Errorf("expected empty old, new set; got old=%q new=%q", cr.OldChecksum, cr.NewChecksum)
	}
	// Must NOT clone/fetch.
	if conn.logContains("git clone") || conn.logContains("fetch") {
		t.Errorf("Check must be read-only; log=%v", conn.execLog)
	}
}

func TestCheck_NoChange(t *testing.T) {
	sha := "abcabcabcabcabcabcabcabcabcabcabcabcabca"
	conn := newRepoMock(true, sha, sha, false)
	m := &Module{}
	cr, err := m.Check(context.Background(), conn, map[string]any{
		"repo": "r", "dest": "/opt/app", "version": "main",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cr.WouldChange {
		t.Error("expected WouldChange=false")
	}
}

func TestCheck_WouldUpdate(t *testing.T) {
	oldSHA := "1111111111111111111111111111111111111111"
	newSHA := "2222222222222222222222222222222222222222"
	conn := newRepoMock(true, oldSHA, newSHA, false)
	m := &Module{}
	cr, err := m.Check(context.Background(), conn, map[string]any{
		"repo": "r", "dest": "/opt/app", "version": "main",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !cr.WouldChange {
		t.Error("expected WouldChange=true")
	}
	if cr.OldChecksum != oldSHA || cr.NewChecksum != newSHA {
		t.Errorf("bad checksums: old=%q new=%q", cr.OldChecksum, cr.NewChecksum)
	}
	if conn.logContains("git clone") || conn.logContains("fetch") || conn.logContains("checkout --detach") {
		t.Errorf("Check must be read-only; log=%v", conn.execLog)
	}
}
