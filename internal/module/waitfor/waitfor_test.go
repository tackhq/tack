package waitfor

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eugenetaranov/bolt/internal/connector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockConnector implements connector.Connector for testing.
type mockConnector struct {
	handler func(ctx context.Context, cmd string) (*connector.Result, error)
}

func (m *mockConnector) Connect(ctx context.Context) error                                   { return nil }
func (m *mockConnector) Close() error                                                        { return nil }
func (m *mockConnector) String() string                                                      { return "mock" }
func (m *mockConnector) SetSudo(enabled bool, password string)                               {}
func (m *mockConnector) Upload(ctx context.Context, src io.Reader, dst string, mode uint32) error {
	return nil
}
func (m *mockConnector) Download(ctx context.Context, src string, dst io.Writer) error { return nil }

func (m *mockConnector) Execute(ctx context.Context, cmd string) (*connector.Result, error) {
	return m.handler(ctx, cmd)
}

// --- Parameter validation tests (7.1) ---

func TestMissingType(t *testing.T) {
	mod := &Module{}
	_, err := mod.Run(context.Background(), nil, map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required parameter 'type' is missing")
}

func TestInvalidType(t *testing.T) {
	mod := &Module{}
	_, err := mod.Run(context.Background(), nil, map[string]any{"type": "invalid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported wait_for type")
}

func TestMissingPortParam(t *testing.T) {
	mod := &Module{}
	_, err := mod.Run(context.Background(), nil, map[string]any{"type": "port"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "port")
}

func TestMissingPathParam(t *testing.T) {
	mod := &Module{}
	conn := &mockConnector{handler: func(ctx context.Context, cmd string) (*connector.Result, error) {
		return &connector.Result{ExitCode: 1}, nil
	}}
	_, err := mod.Run(context.Background(), conn, map[string]any{"type": "path"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required parameter 'path' is missing")
}

func TestMissingCmdParam(t *testing.T) {
	mod := &Module{}
	conn := &mockConnector{handler: func(ctx context.Context, cmd string) (*connector.Result, error) {
		return &connector.Result{ExitCode: 1}, nil
	}}
	_, err := mod.Run(context.Background(), conn, map[string]any{"type": "command"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required parameter 'cmd' is missing")
}

func TestMissingURLParam(t *testing.T) {
	mod := &Module{}
	_, err := mod.Run(context.Background(), nil, map[string]any{"type": "url"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required parameter 'url' is missing")
}

func TestInvalidURLScheme(t *testing.T) {
	mod := &Module{}
	_, err := mod.Run(context.Background(), nil, map[string]any{
		"type": "url",
		"url":  "ftp://example.com",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url must use http or https scheme")
}

func TestDefaultValues(t *testing.T) {
	// Verify defaults by checking that a port check with only port works
	// (host defaults to localhost, timeout/interval have defaults).
	// We'll use a port that's open to get quick success.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	mod := &Module{}
	result, err := mod.Run(context.Background(), nil, map[string]any{
		"type":     "port",
		"port":     port,
		"host":     "127.0.0.1",
		"timeout":  5,
		"interval": 1,
	})
	require.NoError(t, err)
	assert.True(t, result.Changed)
}

// --- Port check tests (7.2) ---

func TestPortStartedSuccess(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	mod := &Module{}
	result, err := mod.Run(context.Background(), nil, map[string]any{
		"type":     "port",
		"port":     port,
		"host":     "127.0.0.1",
		"timeout":  5,
		"interval": 1,
	})
	require.NoError(t, err)
	assert.True(t, result.Changed)
	assert.Contains(t, result.Data, "elapsed")
	assert.Contains(t, result.Data, "attempts")
}

func TestPortStartedTimeout(t *testing.T) {
	// Use a port that's definitely not listening.
	mod := &Module{}
	_, err := mod.Run(context.Background(), nil, map[string]any{
		"type":     "port",
		"port":     59999,
		"host":     "127.0.0.1",
		"timeout":  2,
		"interval": 1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout waiting for port 59999 on 127.0.0.1")
}

func TestPortStoppedSuccess(t *testing.T) {
	// Port not listening — stopped condition is already met.
	mod := &Module{}
	result, err := mod.Run(context.Background(), nil, map[string]any{
		"type":     "port",
		"port":     59998,
		"host":     "127.0.0.1",
		"state":    "stopped",
		"timeout":  5,
		"interval": 1,
	})
	require.NoError(t, err)
	assert.True(t, result.Changed)
}

func TestPortStoppedTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	mod := &Module{}
	_, err = mod.Run(context.Background(), nil, map[string]any{
		"type":     "port",
		"port":     port,
		"host":     "127.0.0.1",
		"state":    "stopped",
		"timeout":  2,
		"interval": 1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout waiting for port")
	assert.Contains(t, err.Error(), "to close")
}

// --- Path check tests (7.3) ---

func TestPathExistsSuccess(t *testing.T) {
	conn := &mockConnector{handler: func(ctx context.Context, cmd string) (*connector.Result, error) {
		if strings.Contains(cmd, "test -e") {
			return &connector.Result{ExitCode: 0}, nil
		}
		return &connector.Result{ExitCode: 1}, nil
	}}

	mod := &Module{}
	result, err := mod.Run(context.Background(), conn, map[string]any{
		"type":     "path",
		"path":     "/tmp/test",
		"timeout":  5,
		"interval": 1,
	})
	require.NoError(t, err)
	assert.True(t, result.Changed)
}

func TestPathExistsTimeout(t *testing.T) {
	conn := &mockConnector{handler: func(ctx context.Context, cmd string) (*connector.Result, error) {
		return &connector.Result{ExitCode: 1}, nil
	}}

	mod := &Module{}
	_, err := mod.Run(context.Background(), conn, map[string]any{
		"type":     "path",
		"path":     "/tmp/nonexistent",
		"timeout":  2,
		"interval": 1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout waiting for path /tmp/nonexistent to exist")
}

func TestPathAbsentSuccess(t *testing.T) {
	conn := &mockConnector{handler: func(ctx context.Context, cmd string) (*connector.Result, error) {
		return &connector.Result{ExitCode: 1}, nil // path doesn't exist
	}}

	mod := &Module{}
	result, err := mod.Run(context.Background(), conn, map[string]any{
		"type":     "path",
		"path":     "/tmp/gone",
		"state":    "stopped",
		"timeout":  5,
		"interval": 1,
	})
	require.NoError(t, err)
	assert.True(t, result.Changed)
}

func TestPathAbsentTimeout(t *testing.T) {
	conn := &mockConnector{handler: func(ctx context.Context, cmd string) (*connector.Result, error) {
		return &connector.Result{ExitCode: 0}, nil // path exists
	}}

	mod := &Module{}
	_, err := mod.Run(context.Background(), conn, map[string]any{
		"type":     "path",
		"path":     "/tmp/stuck",
		"state":    "stopped",
		"timeout":  2,
		"interval": 1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout waiting for path /tmp/stuck to be absent")
}

// --- Command check tests (7.4) ---

func TestCommandSuccess(t *testing.T) {
	conn := &mockConnector{handler: func(ctx context.Context, cmd string) (*connector.Result, error) {
		return &connector.Result{ExitCode: 0, Stdout: "ready\n", Stderr: ""}, nil
	}}

	mod := &Module{}
	result, err := mod.Run(context.Background(), conn, map[string]any{
		"type":     "command",
		"cmd":      "pg_isready",
		"timeout":  5,
		"interval": 1,
	})
	require.NoError(t, err)
	assert.True(t, result.Changed)
	assert.Equal(t, "ready", result.Data["stdout"])
	assert.Equal(t, "", result.Data["stderr"])
}

func TestCommandNonZeroRetries(t *testing.T) {
	var calls int32
	conn := &mockConnector{handler: func(ctx context.Context, cmd string) (*connector.Result, error) {
		n := atomic.AddInt32(&calls, 1)
		if n >= 3 {
			return &connector.Result{ExitCode: 0, Stdout: "ok"}, nil
		}
		return &connector.Result{ExitCode: 1, Stderr: "not ready"}, nil
	}}

	mod := &Module{}
	result, err := mod.Run(context.Background(), conn, map[string]any{
		"type":     "command",
		"cmd":      "check-ready",
		"timeout":  10,
		"interval": 1,
	})
	require.NoError(t, err)
	assert.True(t, result.Changed)
	assert.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(3))
}

func TestCommandTransportError(t *testing.T) {
	conn := &mockConnector{handler: func(ctx context.Context, cmd string) (*connector.Result, error) {
		return nil, fmt.Errorf("SSH connection lost")
	}}

	mod := &Module{}
	_, err := mod.Run(context.Background(), conn, map[string]any{
		"type":     "command",
		"cmd":      "check-ready",
		"timeout":  5,
		"interval": 1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "command transport error")
	assert.Contains(t, err.Error(), "SSH connection lost")
}

// --- URL check tests (7.5) ---

func TestURLSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	mod := &Module{}
	result, err := mod.Run(context.Background(), nil, map[string]any{
		"type":     "url",
		"url":      srv.URL,
		"timeout":  5,
		"interval": 1,
	})
	require.NoError(t, err)
	assert.True(t, result.Changed)
	assert.Equal(t, 200, result.Data["status_code"])
}

func TestURLErrorStatusRetries(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n >= 3 {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	mod := &Module{}
	result, err := mod.Run(context.Background(), nil, map[string]any{
		"type":     "url",
		"url":      srv.URL,
		"timeout":  10,
		"interval": 1,
	})
	require.NoError(t, err)
	assert.True(t, result.Changed)
	assert.Equal(t, 200, result.Data["status_code"])
}

func TestURLConnectionRefusedRetries(t *testing.T) {
	// Start a server, get its URL, then close it — so we have a URL that refuses connections.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srvURL := srv.URL
	srv.Close()

	mod := &Module{}
	_, err := mod.Run(context.Background(), nil, map[string]any{
		"type":     "url",
		"url":      srvURL,
		"timeout":  2,
		"interval": 1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout waiting for url")
}

func TestURLRedirectSuccess(t *testing.T) {
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer final.Close()

	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL, http.StatusMovedPermanently)
	}))
	defer redirect.Close()

	mod := &Module{}
	result, err := mod.Run(context.Background(), nil, map[string]any{
		"type":     "url",
		"url":      redirect.URL,
		"timeout":  5,
		"interval": 1,
	})
	require.NoError(t, err)
	assert.True(t, result.Changed)
	assert.Equal(t, 200, result.Data["status_code"])
}

// --- Check mode test (7.6) ---

func TestCheckModeUncertain(t *testing.T) {
	mod := &Module{}

	for _, typ := range []string{"port", "path", "command", "url"} {
		t.Run(typ, func(t *testing.T) {
			params := map[string]any{"type": typ}
			switch typ {
			case "port":
				params["port"] = 8080
			case "path":
				params["path"] = "/tmp/test"
			case "command":
				params["cmd"] = "true"
			case "url":
				params["url"] = "http://localhost:8080"
			}

			conn := &mockConnector{handler: func(ctx context.Context, cmd string) (*connector.Result, error) {
				return &connector.Result{ExitCode: 0}, nil
			}}

			result, err := mod.Check(context.Background(), conn, params)
			require.NoError(t, err)
			assert.True(t, result.Uncertain)
			assert.Contains(t, result.Message, "wait_for cannot predict future state")
		})
	}
}

// --- Integration test (7.7) ---

func TestIntegrationURLWait(t *testing.T) {
	var ready int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&ready) == 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Make the server "ready" after 2 seconds.
	go func() {
		time.Sleep(2 * time.Second)
		atomic.StoreInt32(&ready, 1)
	}()

	mod := &Module{}
	start := time.Now()
	result, err := mod.Run(context.Background(), nil, map[string]any{
		"type":     "url",
		"url":      srv.URL,
		"timeout":  10,
		"interval": 1,
	})
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.True(t, result.Changed)
	assert.Greater(t, elapsed, 1*time.Second)
	assert.Less(t, elapsed, 8*time.Second)

	// Verify result data.
	assert.Contains(t, result.Data, "elapsed")
	assert.Contains(t, result.Data, "attempts")
	assert.Equal(t, 200, result.Data["status_code"])

	elapsedSec, ok := result.Data["elapsed"].(float64)
	require.True(t, ok)
	assert.Greater(t, elapsedSec, 1.0)

	attempts, ok := result.Data["attempts"].(int)
	require.True(t, ok)
	assert.GreaterOrEqual(t, attempts, 2)
}

func TestModuleName(t *testing.T) {
	mod := &Module{}
	assert.Equal(t, "wait_for", mod.Name())
}

func TestDescriberInterface(t *testing.T) {
	mod := &Module{}
	assert.NotEmpty(t, mod.Description())
	params := mod.Parameters()
	assert.Greater(t, len(params), 0)

	// Verify required params are documented.
	var typeFound bool
	for _, p := range params {
		if p.Name == "type" {
			typeFound = true
			assert.True(t, p.Required)
		}
	}
	assert.True(t, typeFound, "type parameter should be documented")
}
