package inventory

import (
	"context"
	"fmt"
	"sync"
)

// Plugin is the interface that all inventory plugins must implement.
type Plugin interface {
	// Name returns the unique name of the plugin (e.g., "script", "http", "ec2").
	Name() string

	// Load resolves inventory from the plugin's source.
	// The config parameter contains plugin-specific configuration parsed from YAML.
	Load(ctx context.Context, config map[string]any) (*Inventory, error)
}

var (
	pluginsMu sync.RWMutex
	plugins   = make(map[string]Plugin)
)

// RegisterPlugin registers an inventory plugin by name.
func RegisterPlugin(p Plugin) {
	pluginsMu.Lock()
	defer pluginsMu.Unlock()
	plugins[p.Name()] = p
}

// GetPlugin returns a registered plugin by name, or an error if not found.
func GetPlugin(name string) (Plugin, error) {
	pluginsMu.RLock()
	defer pluginsMu.RUnlock()
	p, ok := plugins[name]
	if !ok {
		return nil, fmt.Errorf("unknown inventory plugin %q", name)
	}
	return p, nil
}
