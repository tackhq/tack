// Package ssh provides a connector for executing commands on remote hosts via SSH.
package ssh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/eugenetaranov/bolt/internal/connector"
	sshconfig "github.com/kevinburke/ssh_config"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Default SSH settings.
const (
	defaultPort    = 22
	defaultTimeout = 30 * time.Second
)

// defaultKeyFiles are the private key paths to try, relative to ~/.ssh/.
var defaultKeyFiles = []string{
	"id_ed25519",
	"id_rsa",
	"id_ecdsa",
}

// Connector executes commands on remote hosts via SSH.
type Connector struct {
	host            string
	hostname        string
	user            string
	port            int
	keyFile         string
	password        string
	timeout         time.Duration
	sudo            bool
	sudoUser        string
	insecureHostKey bool

	client       *ssh.Client
	sftpClient   *sftp.Client
	authWarnings []string
}

// Option configures the SSH connector.
type Option func(*Connector)

// WithUser overrides the SSH config user.
func WithUser(user string) Option {
	return func(c *Connector) {
		c.user = user
	}
}

// WithPort overrides the SSH config port.
func WithPort(port int) Option {
	return func(c *Connector) {
		c.port = port
	}
}

// WithKeyFile sets an explicit private key path.
func WithKeyFile(path string) Option {
	return func(c *Connector) {
		c.keyFile = path
	}
}

// WithPassword enables password authentication.
func WithPassword(password string) Option {
	return func(c *Connector) {
		c.password = password
	}
}

// WithTimeout sets the connection timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Connector) {
		c.timeout = d
	}
}

// WithInsecureHostKey skips SSH host key verification.
func WithInsecureHostKey() Option {
	return func(c *Connector) {
		c.insecureHostKey = true
	}
}

// WithSudo enables sudo for command execution.
func WithSudo(user string) Option {
	return func(c *Connector) {
		c.sudo = true
		c.sudoUser = user
	}
}

// New creates a new SSH connector for the specified host.
// The host is looked up in ~/.ssh/config to resolve connection parameters.
func New(host string, opts ...Option) *Connector {
	c := &Connector{
		host:    host,
		timeout: defaultTimeout,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Connect establishes an SSH connection to the target host.
func (c *Connector) Connect(ctx context.Context) error {
	// Resolve SSH config for the host alias
	c.resolveSSHConfig()

	// Build authentication methods
	authMethods := c.buildAuthMethods()
	if len(authMethods) == 0 {
		return fmt.Errorf("no SSH authentication methods available for %s", c.host)
	}

	// Build host key callback
	var hostKeyCallback ssh.HostKeyCallback
	if c.insecureHostKey {
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	} else {
		var err error
		hostKeyCallback, err = buildHostKeyCallback()
		if err != nil {
			// Fall back to insecure if known_hosts is unavailable
			hostKeyCallback = ssh.InsecureIgnoreHostKey()
		}
	}

	config := &ssh.ClientConfig{
		User:            c.user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         c.timeout,
	}

	addr := net.JoinHostPort(c.hostname, strconv.Itoa(c.port))

	// Dial with context support
	dialer := net.Dialer{Timeout: c.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to dial %s: %w", addr, err)
	}

	// Perform SSH handshake
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		conn.Close()
		msg := fmt.Sprintf("SSH handshake failed for %s (user=%s): %v", addr, c.user, err)
		if len(c.authWarnings) > 0 {
			msg += "\n  auth warnings:"
			for _, w := range c.authWarnings {
				msg += "\n    - " + w
			}
		}
		return fmt.Errorf("%s", msg)
	}

	c.client = ssh.NewClient(sshConn, chans, reqs)
	return nil
}

// Execute runs a command on the remote host and returns the result.
func (c *Connector) Execute(ctx context.Context, cmd string) (*connector.Result, error) {
	if c.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	session, err := c.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Build the command with sudo if configured
	fullCmd := c.buildCommand(cmd)

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// Run with context cancellation support
	done := make(chan error, 1)
	go func() {
		done <- session.Run(fullCmd)
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGKILL)
		return nil, ctx.Err()
	case err := <-done:
		result := &connector.Result{
			Stdout: stdout.String(),
			Stderr: stderr.String(),
		}

		if err != nil {
			if exitErr, ok := err.(*ssh.ExitError); ok {
				result.ExitCode = exitErr.ExitStatus()
			} else {
				return nil, fmt.Errorf("failed to execute command: %w", err)
			}
		}

		return result, nil
	}
}

// Upload copies content from src to a remote file at dst using SFTP.
func (c *Connector) Upload(ctx context.Context, src io.Reader, dst string, mode uint32) error {
	if c.client == nil {
		return fmt.Errorf("not connected")
	}

	sftpClient, err := c.getSFTPClient()
	if err != nil {
		return err
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	f, err := sftpClient.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return fmt.Errorf("failed to create remote file %s: %w", dst, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, src); err != nil {
		return fmt.Errorf("failed to write to remote file %s: %w", dst, err)
	}

	if err := sftpClient.Chmod(dst, os.FileMode(mode)); err != nil {
		return fmt.Errorf("failed to set permissions on %s: %w", dst, err)
	}

	return nil
}

// Download copies content from a remote file at src to dst using SFTP.
func (c *Connector) Download(ctx context.Context, src string, dst io.Writer) error {
	if c.client == nil {
		return fmt.Errorf("not connected")
	}

	sftpClient, err := c.getSFTPClient()
	if err != nil {
		return err
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	f, err := sftpClient.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open remote file %s: %w", src, err)
	}
	defer f.Close()

	if _, err := io.Copy(dst, f); err != nil {
		return fmt.Errorf("failed to read remote file %s: %w", src, err)
	}

	return nil
}

// Close terminates the SFTP client and SSH connection.
func (c *Connector) Close() error {
	if c.sftpClient != nil {
		c.sftpClient.Close()
		c.sftpClient = nil
	}
	if c.client != nil {
		err := c.client.Close()
		c.client = nil
		return err
	}
	return nil
}

// String returns a human-readable description of the connection.
func (c *Connector) String() string {
	host := c.hostname
	if host == "" {
		host = c.host
	}
	port := c.port
	if port == 0 {
		port = defaultPort
	}
	desc := fmt.Sprintf("ssh://%s@%s:%d", c.user, host, port)
	if c.sudo && c.sudoUser != "" {
		desc += fmt.Sprintf(" (sudo as %s)", c.sudoUser)
	} else if c.sudo {
		desc += " (sudo)"
	}
	return desc
}

// resolveSSHConfig reads ~/.ssh/config and resolves connection parameters.
// Explicit options set via WithUser/WithPort/etc. take precedence.
func (c *Connector) resolveSSHConfig() {
	// Load SSH config
	configPath := filepath.Join(homeDir(), ".ssh", "config")
	f, err := os.Open(configPath)
	if err != nil {
		// No SSH config file — use defaults
		c.applyDefaults()
		return
	}
	defer f.Close()

	cfg, err := sshconfig.Decode(f)
	if err != nil {
		c.applyDefaults()
		return
	}

	// Resolve hostname (SSH config HostName directive)
	if c.hostname == "" {
		hostname, _ := cfg.Get(c.host, "HostName")
		if hostname != "" {
			c.hostname = hostname
		} else {
			c.hostname = c.host
		}
	}

	// Resolve user
	if c.user == "" {
		configUser, _ := cfg.Get(c.host, "User")
		if configUser != "" {
			c.user = configUser
		}
	}

	// Resolve port
	if c.port == 0 {
		portStr, _ := cfg.Get(c.host, "Port")
		if portStr != "" {
			if p, err := strconv.Atoi(portStr); err == nil {
				c.port = p
			}
		}
	}

	// Resolve identity file
	if c.keyFile == "" {
		identityFile, _ := cfg.Get(c.host, "IdentityFile")
		if identityFile != "" {
			c.keyFile = expandPath(identityFile)
		}
	}

	c.applyDefaults()
}

// applyDefaults fills in any remaining unset fields with defaults.
func (c *Connector) applyDefaults() {
	if c.hostname == "" {
		c.hostname = c.host
	}
	if c.user == "" {
		if u, err := user.Current(); err == nil {
			c.user = u.Username
		}
	}
	if c.port == 0 {
		c.port = defaultPort
	}
}

// buildAuthMethods constructs SSH authentication methods in priority order.
func (c *Connector) buildAuthMethods() []ssh.AuthMethod {
	var methods []ssh.AuthMethod
	c.authWarnings = nil

	// 1. SSH Agent
	if agentAuth := c.sshAgentAuth(); agentAuth != nil {
		methods = append(methods, agentAuth)
	}

	// 2. Key files
	if keyAuth := c.keyFileAuth(); keyAuth != nil {
		methods = append(methods, keyAuth)
	}

	// 3. Password
	if c.password != "" {
		methods = append(methods, ssh.Password(c.password))
	}

	return methods
}

// sshAgentAuth returns an SSH agent auth method if SSH_AUTH_SOCK is available.
func (c *Connector) sshAgentAuth() ssh.AuthMethod {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		c.authWarnings = append(c.authWarnings, "SSH agent not available (SSH_AUTH_SOCK not set)")
		return nil
	}

	conn, err := net.Dial("unix", sock)
	if err != nil {
		c.authWarnings = append(c.authWarnings, fmt.Sprintf("SSH agent connection failed: %v", err))
		return nil
	}

	agentClient := agent.NewClient(conn)
	keys, err := agentClient.List()
	if err != nil || len(keys) == 0 {
		c.authWarnings = append(c.authWarnings, "SSH agent has no identities (try ssh-add)")
		return nil
	}

	return ssh.PublicKeysCallback(agentClient.Signers)
}

// keyFileAuth returns a public key auth method from key files.
func (c *Connector) keyFileAuth() ssh.AuthMethod {
	var signers []ssh.Signer

	// Try explicit key file first
	if c.keyFile != "" {
		path := expandPath(c.keyFile)
		signer, err := loadKey(path)
		if err != nil {
			c.authWarnings = append(c.authWarnings, fmt.Sprintf("key %s: %v", path, err))
		} else if signer != nil {
			signers = append(signers, signer)
		}
	}

	// Try default key files
	sshDir := filepath.Join(homeDir(), ".ssh")
	for _, name := range defaultKeyFiles {
		path := filepath.Join(sshDir, name)
		signer, err := loadKey(path)
		if err != nil {
			c.authWarnings = append(c.authWarnings, fmt.Sprintf("key %s: %v", path, err))
		} else if signer != nil {
			signers = append(signers, signer)
		}
	}

	if len(signers) == 0 {
		return nil
	}

	return ssh.PublicKeys(signers...)
}

// loadKey loads a private key from the given path.
// Returns (nil, nil) if the file does not exist.
func loadKey(path string) (ssh.Signer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read failed: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("passphrase-protected or invalid key")
	}

	return signer, nil
}

// buildCommand wraps the command with sudo if configured.
func (c *Connector) buildCommand(cmd string) string {
	if !c.sudo {
		return cmd
	}

	if c.sudoUser != "" {
		return fmt.Sprintf("sudo -u %s -- %s", c.sudoUser, cmd)
	}
	return fmt.Sprintf("sudo -- %s", cmd)
}

// getSFTPClient returns a cached SFTP client or creates a new one.
func (c *Connector) getSFTPClient() (*sftp.Client, error) {
	if c.sftpClient != nil {
		return c.sftpClient, nil
	}

	client, err := sftp.NewClient(c.client)
	if err != nil {
		return nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}

	c.sftpClient = client
	return client, nil
}

// buildHostKeyCallback creates a known_hosts callback from ~/.ssh/known_hosts.
func buildHostKeyCallback() (ssh.HostKeyCallback, error) {
	knownHostsPath := filepath.Join(homeDir(), ".ssh", "known_hosts")
	return knownhosts.New(knownHostsPath)
}

// homeDir returns the current user's home directory.
func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	if u, err := user.Current(); err == nil {
		return u.HomeDir
	}
	return ""
}

// expandPath expands ~ to the home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir(), path[2:])
	}
	return path
}

// Ensure Connector implements the connector.Connector interface.
var _ connector.Connector = (*Connector)(nil)
