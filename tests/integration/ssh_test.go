package integration

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/crypto/ssh"
)

func setupSSHContainer(t *testing.T, ctx context.Context) testcontainers.Container {
	t.Helper()

	// Remove any existing container with this name
	cmd := exec.Command("docker", "rm", "-f", "bolt-ssh-integration-test")
	_ = cmd.Run()

	dockerfilePath := filepath.Join(projectRoot, "tests", "integration")

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    dockerfilePath,
			Dockerfile: "Dockerfile.ssh",
		},
		Name:         "bolt-ssh-integration-test",
		ExposedPorts: []string{"22/tcp"},
		WaitingFor:   wait.ForListeningPort("22/tcp").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "failed to start SSH test container")

	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate SSH container: %v", err)
		}
	})

	return container
}

// generateTestKey creates a temporary ed25519 keypair and injects the public key
// into the container's authorized_keys for testuser. Returns the path to the private key.
func generateTestKey(t *testing.T, ctx context.Context, container testcontainers.Container) string {
	t.Helper()

	// Generate ed25519 key pair
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	// Marshal private key to PEM
	privBytes, err := ssh.MarshalPrivateKey(privKey, "")
	require.NoError(t, err)

	keyDir := t.TempDir()
	keyPath := filepath.Join(keyDir, "id_ed25519")
	err = os.WriteFile(keyPath, pem.EncodeToMemory(privBytes), 0600)
	require.NoError(t, err)

	// Build authorized_keys line from public key
	sshPub, err := ssh.NewPublicKey(pubKey)
	require.NoError(t, err)
	authorizedKey := string(ssh.MarshalAuthorizedKey(sshPub))

	// Inject into container — write via tee to avoid shell quoting issues
	pubKeyLine := strings.TrimSpace(authorizedKey)
	exitCode, _, err := execInContainer(ctx, container, []string{
		"bash", "-c",
		"echo '" + pubKeyLine + "' >> /home/testuser/.ssh/authorized_keys && chown testuser:testuser /home/testuser/.ssh/authorized_keys",
	})
	require.NoError(t, err)
	require.Equal(t, 0, exitCode, "failed to inject public key into container")

	return keyPath
}

func getSSHPort(t *testing.T, ctx context.Context, container testcontainers.Container) string {
	t.Helper()
	mappedPort, err := container.MappedPort(ctx, "22")
	require.NoError(t, err)
	return mappedPort.Port()
}

func TestSSHIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	container := setupSSHContainer(t, ctx)
	port := getSSHPort(t, ctx, container)
	keyPath := generateTestKey(t, ctx, container)
	playbookPath := filepath.Join(projectRoot, "tests", "integration", "testdata", "ssh-playbook.yaml")

	t.Run("PasswordAuth", func(t *testing.T) {
		cmd := exec.Command(boltBinaryPath, "run", playbookPath,
			"--connection", "ssh",
			"--hosts", "127.0.0.1",
			"--ssh-port", port,
			"--ssh-user", "testuser",
			"--ssh-password", "testpass",
			"--ssh-insecure",
		)
		cmd.Dir = projectRoot
		// Clear SSH_AUTH_SOCK so agent doesn't interfere
		cmd.Env = filterEnv(os.Environ(), "SSH_AUTH_SOCK")
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "password auth failed: %s", string(output))
		t.Logf("PasswordAuth output:\n%s", string(output))

		// Verify marker file was created
		assertFileExists(t, ctx, container, "/tmp/ssh-marker.txt")
		assertFileContains(t, ctx, container, "/tmp/ssh-marker.txt", []string{"ssh-ok"})

		// Clean up marker for next test
		_, _, _ = execInContainer(ctx, container, []string{"rm", "-f", "/tmp/ssh-marker.txt"})
	})

	t.Run("KeyAuth", func(t *testing.T) {
		cmd := exec.Command(boltBinaryPath, "run", playbookPath,
			"--connection", "ssh",
			"--hosts", "127.0.0.1",
			"--ssh-port", port,
			"--ssh-user", "testuser",
			"--ssh-key", keyPath,
			"--ssh-insecure",
		)
		cmd.Dir = projectRoot
		cmd.Env = filterEnv(os.Environ(), "SSH_AUTH_SOCK")
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "key auth failed: %s", string(output))
		t.Logf("KeyAuth output:\n%s", string(output))

		assertFileExists(t, ctx, container, "/tmp/ssh-marker.txt")
		assertFileContains(t, ctx, container, "/tmp/ssh-marker.txt", []string{"ssh-ok"})

		_, _, _ = execInContainer(ctx, container, []string{"rm", "-f", "/tmp/ssh-marker.txt"})
	})

	t.Run("KeyAuthWithEmptyAgent", func(t *testing.T) {
		// Regression test: when the SSH agent is running but has no identities,
		// key file auth must still work. Previously the agent's empty publickey
		// method consumed the server's publickey allowance, causing the key file
		// method to be rejected.
		agentSock := startEmptyAgent(t)

		cmd := exec.Command(boltBinaryPath, "run", playbookPath,
			"--connection", "ssh",
			"--hosts", "127.0.0.1",
			"--ssh-port", port,
			"--ssh-user", "testuser",
			"--ssh-key", keyPath,
			"--ssh-insecure",
		)
		cmd.Dir = projectRoot
		// Use the empty agent instead of the host agent, and isolate HOME
		// so no default keys from ~/.ssh/ are found.
		fakeHome := t.TempDir()
		env := filterEnv(os.Environ(), "SSH_AUTH_SOCK", "HOME")
		env = append(env, "SSH_AUTH_SOCK="+agentSock, "HOME="+fakeHome)
		cmd.Env = env

		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "key auth with empty agent failed: %s", string(output))
		t.Logf("KeyAuthWithEmptyAgent output:\n%s", string(output))

		assertFileExists(t, ctx, container, "/tmp/ssh-marker.txt")
		assertFileContains(t, ctx, container, "/tmp/ssh-marker.txt", []string{"ssh-ok"})

		_, _, _ = execInContainer(ctx, container, []string{"rm", "-f", "/tmp/ssh-marker.txt"})
	})

	t.Run("InsecureHostKey", func(t *testing.T) {
		// Connect with --ssh-insecure (no known_hosts needed)
		cmd := exec.Command(boltBinaryPath, "run", playbookPath,
			"--connection", "ssh",
			"--hosts", "127.0.0.1",
			"--ssh-port", port,
			"--ssh-user", "testuser",
			"--ssh-password", "testpass",
			"--ssh-insecure",
		)
		cmd.Dir = projectRoot
		cmd.Env = filterEnv(os.Environ(), "SSH_AUTH_SOCK")
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "insecure host key connection failed: %s", string(output))
		t.Logf("InsecureHostKey output:\n%s", string(output))

		_, _, _ = execInContainer(ctx, container, []string{"rm", "-f", "/tmp/ssh-marker.txt"})
	})

	t.Run("HostKeyRejected", func(t *testing.T) {
		// Set HOME to a temp dir with an empty known_hosts so the host key is unknown.
		// The SSH connector only falls back to insecure when known_hosts doesn't exist;
		// an empty file means "no hosts trusted" and every key gets rejected.
		fakeHome := t.TempDir()
		sshDir := filepath.Join(fakeHome, ".ssh")
		require.NoError(t, os.MkdirAll(sshDir, 0700))
		require.NoError(t, os.WriteFile(filepath.Join(sshDir, "known_hosts"), []byte{}, 0600))

		cmd := exec.Command(boltBinaryPath, "run", playbookPath,
			"--connection", "ssh",
			"--hosts", "127.0.0.1",
			"--ssh-port", port,
			"--ssh-user", "testuser",
			"--ssh-password", "testpass",
			// Deliberately NO --ssh-insecure
		)
		cmd.Dir = projectRoot
		env := filterEnv(os.Environ(), "SSH_AUTH_SOCK", "HOME")
		env = append(env, "HOME="+fakeHome)
		cmd.Env = env

		output, err := cmd.CombinedOutput()
		require.Error(t, err, "expected host key rejection, but got success: %s", string(output))
		assert.Contains(t, string(output), "handshake failed",
			"error should mention handshake failure: %s", string(output))
		t.Logf("HostKeyRejected output:\n%s", string(output))
	})

	t.Run("WrongUser", func(t *testing.T) {
		cmd := exec.Command(boltBinaryPath, "run", playbookPath,
			"--connection", "ssh",
			"--hosts", "127.0.0.1",
			"--ssh-port", port,
			"--ssh-user", "wronguser",
			"--ssh-password", "testpass",
			"--ssh-insecure",
		)
		cmd.Dir = projectRoot
		cmd.Env = filterEnv(os.Environ(), "SSH_AUTH_SOCK")
		output, err := cmd.CombinedOutput()
		require.Error(t, err, "expected auth failure for wrong user, but got success: %s", string(output))
		assert.Contains(t, string(output), "unable to authenticate",
			"error should mention authentication failure: %s", string(output))
		t.Logf("WrongUser output:\n%s", string(output))
	})

	t.Run("WrongPassword", func(t *testing.T) {
		cmd := exec.Command(boltBinaryPath, "run", playbookPath,
			"--connection", "ssh",
			"--hosts", "127.0.0.1",
			"--ssh-port", port,
			"--ssh-user", "testuser",
			"--ssh-password", "wrongpassword",
			"--ssh-insecure",
		)
		cmd.Dir = projectRoot
		cmd.Env = filterEnv(os.Environ(), "SSH_AUTH_SOCK")
		output, err := cmd.CombinedOutput()
		require.Error(t, err, "expected auth failure for wrong password, but got success: %s", string(output))
		assert.Contains(t, string(output), "unable to authenticate",
			"error should mention authentication failure: %s", string(output))
		t.Logf("WrongPassword output:\n%s", string(output))
	})
}

// startEmptyAgent starts a fresh ssh-agent with no identities and returns its socket path.
// The agent is killed when the test finishes.
func startEmptyAgent(t *testing.T) string {
	t.Helper()

	// Start ssh-agent and parse its output for SSH_AUTH_SOCK
	out, err := exec.Command("ssh-agent", "-s").Output()
	require.NoError(t, err, "failed to start ssh-agent")

	var sock string
	var pid string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "SSH_AUTH_SOCK=") {
			sock = strings.TrimSuffix(strings.TrimPrefix(line, "SSH_AUTH_SOCK="), "; export SSH_AUTH_SOCK;")
		}
		if strings.HasPrefix(line, "SSH_AGENT_PID=") {
			pid = strings.TrimSuffix(strings.TrimPrefix(line, "SSH_AGENT_PID="), "; export SSH_AGENT_PID;")
		}
	}
	require.NotEmpty(t, sock, "could not parse SSH_AUTH_SOCK from ssh-agent output: %s", string(out))

	t.Cleanup(func() {
		if pid != "" {
			killCmd := exec.Command("kill", pid)
			_ = killCmd.Run()
		}
	})

	return sock
}

// filterEnv returns a copy of env with the named variables removed.
func filterEnv(env []string, remove ...string) []string {
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		skip := false
		for _, r := range remove {
			if len(e) > len(r) && e[:len(r)+1] == r+"=" {
				skip = true
				break
			}
		}
		if !skip {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
