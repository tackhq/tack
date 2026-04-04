package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPPlugin_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hosts": {"web1": {"vars": {"env": "prod"}}}, "groups": {"web": {"hosts": ["web1"]}}}`))
	}))
	defer srv.Close()

	p := &Plugin{}
	inv, err := p.Load(context.Background(), map[string]any{"url": srv.URL})
	require.NoError(t, err)
	assert.Len(t, inv.Hosts, 1)
	assert.Equal(t, "prod", inv.Hosts["web1"].Vars["env"])
	assert.Equal(t, []string{"web1"}, inv.Groups["web"].Hosts)
}

func TestHTTPPlugin_YAMLResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hosts:\n  db1:\n    vars:\n      role: database\n"))
	}))
	defer srv.Close()

	p := &Plugin{}
	inv, err := p.Load(context.Background(), map[string]any{"url": srv.URL})
	require.NoError(t, err)
	assert.Equal(t, "database", inv.Hosts["db1"].Vars["role"])
}

func TestHTTPPlugin_QueryParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "prod", r.URL.Query().Get("env"))
		_, _ = w.Write([]byte(`{"hosts": {"ok": {}}}`))
	}))
	defer srv.Close()

	p := &Plugin{}
	inv, err := p.Load(context.Background(), map[string]any{
		"url":    srv.URL,
		"params": map[string]any{"env": "prod"},
	})
	require.NoError(t, err)
	assert.Contains(t, inv.Hosts, "ok")
}

func TestHTTPPlugin_CustomHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "secret123", r.Header.Get("X-API-Key"))
		_, _ = w.Write([]byte(`{"hosts": {"ok": {}}}`))
	}))
	defer srv.Close()

	p := &Plugin{}
	inv, err := p.Load(context.Background(), map[string]any{
		"url":     srv.URL,
		"headers": map[string]any{"X-API-Key": "secret123"},
	})
	require.NoError(t, err)
	assert.Contains(t, inv.Hosts, "ok")
}

func TestHTTPPlugin_BasicAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "admin", user)
		assert.Equal(t, "secret", pass)
		_, _ = w.Write([]byte(`{"hosts": {"ok": {}}}`))
	}))
	defer srv.Close()

	p := &Plugin{}
	inv, err := p.Load(context.Background(), map[string]any{
		"url": srv.URL,
		"auth": map[string]any{
			"basic": map[string]any{
				"username": "admin",
				"password": "secret",
			},
		},
	})
	require.NoError(t, err)
	assert.Contains(t, inv.Hosts, "ok")
}

func TestHTTPPlugin_Non2xxStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("access denied"))
	}))
	defer srv.Close()

	p := &Plugin{}
	_, err := p.Load(context.Background(), map[string]any{"url": srv.URL})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
	assert.Contains(t, err.Error(), "access denied")
}

func TestHTTPPlugin_MissingURL(t *testing.T) {
	p := &Plugin{}
	_, err := p.Load(context.Background(), map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required")
}

func TestHTTPPlugin_EnvInterpolation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token-123", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"hosts": {"ok": {}}}`))
	}))
	defer srv.Close()

	t.Setenv("TEST_API_TOKEN", "test-token-123")

	p := &Plugin{}
	inv, err := p.Load(context.Background(), map[string]any{
		"url":     srv.URL,
		"headers": map[string]any{"Authorization": "Bearer {{ env.TEST_API_TOKEN }}"},
	})
	require.NoError(t, err)
	assert.Contains(t, inv.Hosts, "ok")
}

func TestHTTPPlugin_InsecureSkipVerify(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"hosts": {"tls-ok": {}}}`))
	}))
	defer srv.Close()

	p := &Plugin{}
	inv, err := p.Load(context.Background(), map[string]any{
		"url": srv.URL,
		"tls": map[string]any{"insecure_skip_verify": true},
	})
	require.NoError(t, err)
	assert.Contains(t, inv.Hosts, "tls-ok")
}

func TestInterpolateEnv(t *testing.T) {
	t.Setenv("MY_VAR", "hello")

	assert.Equal(t, "hello", interpolateEnv("{{ env.MY_VAR }}"))
	assert.Equal(t, "Bearer hello", interpolateEnv("Bearer {{ env.MY_VAR }}"))
	assert.Equal(t, "no-change", interpolateEnv("no-change"))
	// Undefined var stays as-is
	assert.Equal(t, "{{ env.UNDEFINED_VAR_XYZ }}", interpolateEnv("{{ env.UNDEFINED_VAR_XYZ }}"))
}
