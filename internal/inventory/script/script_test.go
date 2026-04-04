package script

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eugenetaranov/bolt/internal/inventory"
)

func writeScript(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0755)
	require.NoError(t, err)
	return path
}

func TestScriptPlugin_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	path := writeScript(t, dir, "inv.sh", `#!/bin/sh
cat <<'EOF'
{
  "hosts": {
    "web1": {"vars": {"region": "us-east-1"}},
    "web2": {"vars": {"region": "us-west-2"}}
  },
  "groups": {
    "webservers": {"hosts": ["web1", "web2"], "vars": {"port": 8080}}
  }
}
EOF
`)

	p := &Plugin{}
	inv, err := p.Load(context.Background(), map[string]any{"path": path})
	require.NoError(t, err)
	assert.Len(t, inv.Hosts, 2)
	assert.Equal(t, "us-east-1", inv.Hosts["web1"].Vars["region"])
	assert.Len(t, inv.Groups, 1)
	assert.Equal(t, []string{"web1", "web2"}, inv.Groups["webservers"].Hosts)
}

func TestScriptPlugin_YAMLOutput(t *testing.T) {
	dir := t.TempDir()
	path := writeScript(t, dir, "inv.sh", `#!/bin/sh
cat <<'EOF'
hosts:
  db1:
    vars:
      role: database
groups:
  databases:
    hosts: [db1]
EOF
`)

	p := &Plugin{}
	inv, err := p.Load(context.Background(), map[string]any{"path": path})
	require.NoError(t, err)
	assert.Len(t, inv.Hosts, 1)
	assert.Equal(t, "database", inv.Hosts["db1"].Vars["role"])
	assert.Equal(t, []string{"db1"}, inv.Groups["databases"].Hosts)
}

func TestScriptPlugin_NonZeroExit(t *testing.T) {
	dir := t.TempDir()
	path := writeScript(t, dir, "inv.sh", `#!/bin/sh
echo "connection refused" >&2
exit 1
`)

	p := &Plugin{}
	_, err := p.Load(context.Background(), map[string]any{"path": path})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

func TestScriptPlugin_EmptyOutput(t *testing.T) {
	dir := t.TempDir()
	path := writeScript(t, dir, "inv.sh", `#!/bin/sh
exit 0
`)

	p := &Plugin{}
	_, err := p.Load(context.Background(), map[string]any{"path": path})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no output")
}

func TestScriptPlugin_Timeout(t *testing.T) {
	dir := t.TempDir()
	path := writeScript(t, dir, "inv.sh", `#!/bin/sh
sleep 30
`)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	p := &Plugin{}
	_, err := p.Load(ctx, map[string]any{"path": path})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}

func TestScriptPlugin_MissingPath(t *testing.T) {
	p := &Plugin{}
	_, err := p.Load(context.Background(), map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required")
}

func TestScriptPlugin_ReceivesList(t *testing.T) {
	dir := t.TempDir()
	// Script that checks it received --list argument
	path := writeScript(t, dir, "inv.sh", `#!/bin/sh
if [ "$1" = "--list" ]; then
  echo '{"hosts": {"ok": {}}}'
else
  echo "expected --list, got $1" >&2
  exit 1
fi
`)

	p := &Plugin{}
	inv, err := p.Load(context.Background(), map[string]any{"path": path})
	require.NoError(t, err)
	assert.Contains(t, inv.Hosts, "ok")
}

// Test routing: LoadWithContext auto-detects executable
func TestLoadWithContext_ExecutableRouting(t *testing.T) {
	dir := t.TempDir()
	path := writeScript(t, dir, "inv.sh", `#!/bin/sh
echo '{"hosts": {"routed": {}}}'
`)

	inv, err := inventory.LoadWithContext(context.Background(), path)
	require.NoError(t, err)
	assert.Contains(t, inv.Hosts, "routed")
}

// Test routing: static YAML fallback
func TestLoadWithContext_StaticYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts.yml")
	err := os.WriteFile(path, []byte(`
hosts:
  web1:
    vars:
      env: prod
`), 0644)
	require.NoError(t, err)

	inv, err := inventory.LoadWithContext(context.Background(), path)
	require.NoError(t, err)
	assert.Equal(t, "prod", inv.Hosts["web1"].Vars["env"])
}

// Test routing: plugin key dispatch
func TestLoadWithContext_PluginKeyDispatch(t *testing.T) {
	dir := t.TempDir()
	// script plugin is the only one registered; "unknown" should fail
	path := filepath.Join(dir, "inv.yml")
	err := os.WriteFile(path, []byte(`plugin: nonexistent`), 0644)
	require.NoError(t, err)

	_, err = inventory.LoadWithContext(context.Background(), path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown inventory plugin")
}
