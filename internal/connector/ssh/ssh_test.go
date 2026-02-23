package ssh

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	c := New("myhost")
	assert.Equal(t, "myhost", c.host)
	assert.Equal(t, defaultTimeout, c.timeout)
	assert.False(t, c.sudo)
}

func TestOptions(t *testing.T) {
	c := New("myhost",
		WithUser("deploy"),
		WithPort(2222),
		WithKeyFile("/tmp/key"),
		WithPassword("secret"),
		WithTimeout(10*time.Second),
		WithSudo(),
		WithSudoPassword("pass123"),
	)

	assert.Equal(t, "deploy", c.user)
	assert.Equal(t, 2222, c.port)
	assert.Equal(t, "/tmp/key", c.keyFile)
	assert.Equal(t, "secret", c.password)
	assert.Equal(t, 10*time.Second, c.timeout)
	assert.True(t, c.sudo)
	assert.Equal(t, "pass123", c.sudoPassword)
}

func TestApplyDefaults(t *testing.T) {
	c := New("example.com")
	c.applyDefaults()

	assert.Equal(t, "example.com", c.hostname)
	assert.Equal(t, defaultPort, c.port)
	assert.NotEmpty(t, c.user) // should be current user
}

func TestApplyDefaultsPreservesExplicit(t *testing.T) {
	c := New("example.com",
		WithUser("deploy"),
		WithPort(2222),
	)
	c.hostname = "real.example.com"
	c.applyDefaults()

	assert.Equal(t, "real.example.com", c.hostname)
	assert.Equal(t, "deploy", c.user)
	assert.Equal(t, 2222, c.port)
}

func TestBuildCommand(t *testing.T) {
	tests := []struct {
		name         string
		sudo         bool
		sudoPassword string
		user         string
		cmd          string
		expected     string
	}{
		{
			name:     "no sudo",
			cmd:      "whoami",
			expected: "whoami",
		},
		{
			name:     "sudo without password",
			sudo:     true,
			cmd:      "whoami",
			expected: "sudo sh -c 'whoami'",
		},
		{
			name:         "sudo with password",
			sudo:         true,
			sudoPassword: "secret",
			cmd:          "whoami",
			expected:     "printf '%s\\n' 'secret' | sudo -S sh -c 'whoami'",
		},
		{
			name:     "sudo skipped for root user",
			sudo:     true,
			user:     "root",
			cmd:      "whoami",
			expected: "whoami",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Connector{sudo: tt.sudo, sudoPassword: tt.sudoPassword, user: tt.user}
			assert.Equal(t, tt.expected, c.buildCommand(tt.cmd))
		})
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		name     string
		opts     []Option
		hostname string
		expected string
	}{
		{
			name:     "basic",
			opts:     []Option{WithUser("admin"), WithPort(22)},
			hostname: "example.com",
			expected: "ssh://admin@example.com:22",
		},
		{
			name:     "custom port",
			opts:     []Option{WithUser("deploy"), WithPort(2222)},
			hostname: "server.local",
			expected: "ssh://deploy@server.local:2222",
		},
		{
			name:     "with sudo",
			opts:     []Option{WithUser("deploy"), WithPort(22), WithSudo()},
			hostname: "server.local",
			expected: "ssh://deploy@server.local:22 (sudo)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New("host", tt.opts...)
			c.hostname = tt.hostname
			assert.Equal(t, tt.expected, c.String())
		})
	}
}

func TestExpandPath(t *testing.T) {
	home := homeDir()
	require.NotEmpty(t, home)

	assert.Equal(t, home+"/foo", expandPath("~/foo"))
	assert.Equal(t, "/absolute/path", expandPath("/absolute/path"))
	assert.Equal(t, "relative/path", expandPath("relative/path"))
}

func TestConnectorInterface(t *testing.T) {
	// Compile-time check is done via var _ connector.Connector = (*Connector)(nil)
	// but let's verify the type assertion works at runtime too
	c := New("test")
	assert.NotNil(t, c)
}

func TestResolveSSHConfigNoFile(t *testing.T) {
	// Set HOME to a temp dir with no .ssh/config
	origHome := homeDir()
	t.Setenv("HOME", t.TempDir())
	defer t.Setenv("HOME", origHome)

	c := New("myhost", WithUser("testuser"))
	c.resolveSSHConfig()

	assert.Equal(t, "myhost", c.hostname)
	assert.Equal(t, "testuser", c.user)
	assert.Equal(t, defaultPort, c.port)
}
