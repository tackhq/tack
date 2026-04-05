package integration

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGitModule exercises the `git` module end-to-end against a local
// bare repo inside the test container (no network required).
func TestGitModule(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	container := setupTestContainer(t, ctx)

	playbookPath := filepath.Join(projectRoot, "tests", "integration", "testdata", "git_playbook.yaml")
	cmd := exec.Command(tackBinaryPath, "run", playbookPath, "--auto-approve")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git playbook failed: %s", string(output))
	t.Logf("Playbook output:\n%s", string(output))

	// Verify checkout produced a worktree.
	assertFileExists(t, ctx, container, "/tmp/git-dest/README.md")
	assertFileContains(t, ctx, container, "/tmp/git-dest/README.md", []string{"hello"})
	assertIsDirectory(t, ctx, container, "/tmp/git-dest/.git")

	// Verify downstream consumption of register output (after_sha was populated
	// and written via command task).
	assertFileExists(t, ctx, container, "/tmp/git-after-sha.txt")
	_, content, err := execInContainer(ctx, container, []string{"cat", "/tmp/git-after-sha.txt"})
	require.NoError(t, err)
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "after_sha=") || len(content) != len("after_sha=")+40 {
		t.Fatalf("expected after_sha=<40-char-sha>, got %q", content)
	}
}
