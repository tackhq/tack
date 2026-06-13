package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newDiscoveryCmd builds a bare command carrying the skip-discovery flag so
// resolvePlaybookRef can be exercised in isolation.
func newDiscoveryCmd(skip bool) *cobra.Command {
	c := &cobra.Command{Use: "x"}
	c.Flags().Bool("skip-discovery", skip, "")
	return c
}

func writeFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("plays: []\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestDiscoverDefaultFile_None(t *testing.T) {
	t.Chdir(t.TempDir())

	got, err := discoverDefaultFile("playbook", defaultPlaybookNames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty result, got %q", got)
	}
}

func TestDiscoverDefaultFile_Single(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "site.yaml")
	t.Chdir(dir)

	got, err := discoverDefaultFile("playbook", defaultPlaybookNames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "site.yaml" {
		t.Fatalf("expected site.yaml, got %q", got)
	}
}

func TestDiscoverDefaultFile_PrefersYamlOrder(t *testing.T) {
	// Only the .yml variant exists; it should be found.
	dir := t.TempDir()
	writeFile(t, dir, "site.yml")
	t.Chdir(dir)

	got, err := discoverDefaultFile("playbook", defaultPlaybookNames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "site.yml" {
		t.Fatalf("expected site.yml, got %q", got)
	}
}

func TestDiscoverDefaultFile_Ambiguous(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "site.yaml")
	writeFile(t, dir, "site.yml")
	t.Chdir(dir)

	_, err := discoverDefaultFile("playbook", defaultPlaybookNames)
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous default playbook") {
		t.Fatalf("expected ambiguity message, got %v", err)
	}
}

func TestDiscoverDefaultFile_IgnoresDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "site.yaml"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Chdir(dir)

	got, err := discoverDefaultFile("playbook", defaultPlaybookNames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected directory to be ignored, got %q", got)
	}
}

func TestRunCmd_AcceptsZeroArgs(t *testing.T) {
	// The run command should accept zero positional args (default discovery).
	if err := runCmd.Args(runCmd, []string{}); err != nil {
		t.Fatalf("run should accept zero args: %v", err)
	}
	if err := runCmd.Args(runCmd, []string{"a", "b"}); err == nil {
		t.Fatal("run should reject more than one arg")
	}
}

func TestValidateCmd_AcceptsZeroArgs(t *testing.T) {
	// cobra.ArbitraryArgs has a nil Args validator; verify it is not ExactArgs/MinimumNArgs.
	if validateCmd.Args != nil {
		if err := validateCmd.Args(validateCmd, []string{}); err != nil {
			t.Fatalf("validate should accept zero args: %v", err)
		}
	}
}

func TestResolvePlaybookRef_ExplicitArgWins(t *testing.T) {
	// An explicit arg is returned verbatim and skips discovery entirely.
	t.Chdir(t.TempDir())
	got, err := resolvePlaybookRef(newDiscoveryCmd(false), []string{"custom.yaml"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "custom.yaml" {
		t.Fatalf("expected custom.yaml, got %q", got)
	}
}

func TestResolvePlaybookRef_SkipDiscovery(t *testing.T) {
	// With discovery disabled and no arg, a present site.yaml is ignored.
	dir := t.TempDir()
	writeFile(t, dir, "site.yaml")
	t.Chdir(dir)

	_, err := resolvePlaybookRef(newDiscoveryCmd(true), nil)
	if err == nil {
		t.Fatal("expected error when discovery is skipped and no arg given")
	}
	if !strings.Contains(err.Error(), "--skip-discovery") {
		t.Fatalf("expected --skip-discovery in error, got %v", err)
	}
}

func TestResolvePlaybookRef_DiscoversByDefault(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "site.yaml")
	t.Chdir(dir)

	got, err := resolvePlaybookRef(newDiscoveryCmd(false), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "site.yaml" {
		t.Fatalf("expected discovered site.yaml, got %q", got)
	}
}

func TestAutoApprove_ShorthandY(t *testing.T) {
	flag := runCmd.Flags().ShorthandLookup("y")
	if flag == nil || flag.Name != "auto-approve" {
		t.Fatal("expected -y to be the shorthand for --auto-approve on run")
	}
}

func TestSkipDiscovery_FlagPresent(t *testing.T) {
	for _, c := range []*cobra.Command{runCmd, validateCmd, testCmd, exportCmd} {
		if c.Flags().Lookup("skip-discovery") == nil {
			t.Errorf("expected --skip-discovery flag on %q command", c.Name())
		}
	}
}
