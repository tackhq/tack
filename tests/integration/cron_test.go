package integration

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCronModule exercises the `cron` module end-to-end against a Linux
// container with the `cron` package installed. The playbook covers:
// create, idempotent re-run, update, disable/enable, env line, remove,
// and /etc/cron.d drop-in lifecycle (create, idempotency, delete).
func TestCronModule(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	container := setupTestContainer(t, ctx)

	// Sanity-check that the crontab binary is available in the image.
	exitCode, _, err := execInContainer(ctx, container, []string{"which", "crontab"})
	require.NoError(t, err)
	require.Equal(t, 0, exitCode, "crontab binary must be available in the test image")

	playbookPath := filepath.Join(projectRoot, "tests", "integration", "testdata", "cron_playbook.yaml")
	cmd := exec.Command(tackBinaryPath, "run", playbookPath, "--auto-approve")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "cron playbook failed: %s", string(output))
	t.Logf("Playbook output:\n%s", string(output))

	// After remove, the backup entry should be gone but the env line stays.
	_, crontabContent, err := execInContainer(ctx, container, []string{"crontab", "-l"})
	require.NoError(t, err)
	if strings.Contains(crontabContent, "# TACK: backup") {
		t.Errorf("backup marker should have been removed, got crontab:\n%s", crontabContent)
	}
	if !strings.Contains(crontabContent, "# TACK: path") {
		t.Errorf("path env marker should still be present, got crontab:\n%s", crontabContent)
	}
	if !strings.Contains(crontabContent, "PATH=/usr/local/bin:/usr/bin:/bin") {
		t.Errorf("PATH env line should be present, got crontab:\n%s", crontabContent)
	}

	// The drop-in file should be deleted after the absent task.
	exitCode, _, err = execInContainer(ctx, container, []string{"test", "-e", "/etc/cron.d/healthcheck"})
	require.NoError(t, err)
	if exitCode == 0 {
		t.Error("/etc/cron.d/healthcheck should have been deleted")
	}
}
