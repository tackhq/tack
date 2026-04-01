// Package inventory loads and queries host inventory files.
package inventory

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/eugenetaranov/bolt/internal/playbook"
)

// HostEntry defines a single host in the inventory.
type HostEntry struct {
	// SSH holds per-host SSH connection overrides.
	SSH *playbook.SSHConfig `yaml:"ssh"`

	// Vars holds per-host variables available in task templates.
	Vars map[string]any `yaml:"vars"`
}

// GroupSSMConfig holds SSM settings for a group.
type GroupSSMConfig struct {
	Region string `yaml:"region"`
	Bucket string `yaml:"bucket"`

	// Instances lists EC2 instance IDs to target.
	// Convenience alias: these are merged with the group's hosts list.
	Instances []string `yaml:"instances"`

	// Tags selects EC2 instances by tag at runtime (mutually exclusive with Instances).
	Tags map[string]string `yaml:"tags"`
}

// GroupEntry defines a named group of hosts.
type GroupEntry struct {
	// Connection overrides the play's connection type for hosts in this group.
	Connection string `yaml:"connection"`

	// SSH holds group-level SSH defaults (overridden by per-host SSH config).
	SSH *playbook.SSHConfig `yaml:"ssh"`

	// SSM holds group-level SSM defaults.
	SSM *GroupSSMConfig `yaml:"ssm"`

	// Hosts lists the host names (or instance IDs) that belong to this group.
	Hosts []string `yaml:"hosts"`

	// Vars holds group-level variables (lower priority than per-host vars).
	Vars map[string]any `yaml:"vars"`
}

// Inventory holds the parsed inventory.
type Inventory struct {
	// Hosts maps host names to their entries.
	Hosts map[string]*HostEntry `yaml:"hosts"`

	// Groups maps group names to their entries.
	Groups map[string]*GroupEntry `yaml:"groups"`
}

// Load reads and parses an inventory file from path.
func Load(path string) (*Inventory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read inventory file: %w", err)
	}

	inv := &Inventory{}
	if err := yaml.Unmarshal(data, inv); err != nil {
		return nil, fmt.Errorf("failed to parse inventory file: %w", err)
	}

	return inv, nil
}

// ExpandGroup resolves a name to a list of host strings and the matching group
// entry (if any).
//
//   - If name is a known group: returns (group.Hosts, group, true).
//   - If name is a known host (but not a group): returns ([name], nil, true).
//   - Otherwise: returns (nil, nil, false) — caller should pass the name through as-is.
func (inv *Inventory) ExpandGroup(name string) ([]string, *GroupEntry, bool) {
	if inv == nil {
		return nil, nil, false
	}
	if g, ok := inv.Groups[name]; ok {
		// Merge hosts list with ssm.instances (both are valid ways to list targets).
		hosts := append([]string{}, g.Hosts...)
		if g.SSM != nil {
			hosts = append(hosts, g.SSM.Instances...)
		}
		return hosts, g, true
	}
	if _, ok := inv.Hosts[name]; ok {
		return []string{name}, nil, true
	}
	return nil, nil, false
}

// GetHost returns the HostEntry for the given host name, or nil if not defined.
func (inv *Inventory) GetHost(name string) *HostEntry {
	if inv == nil {
		return nil
	}
	return inv.Hosts[name]
}

// AllHosts expands every group, merges top-level hosts, and returns a
// deduplicated list of all host names in the inventory.
func (inv *Inventory) AllHosts() []string {
	if inv == nil {
		return nil
	}
	seen := make(map[string]bool)
	var result []string
	add := func(h string) {
		if !seen[h] {
			seen[h] = true
			result = append(result, h)
		}
	}

	for _, g := range inv.Groups {
		for _, h := range g.Hosts {
			add(h)
		}
		if g.SSM != nil {
			for _, h := range g.SSM.Instances {
				add(h)
			}
		}
	}
	for name := range inv.Hosts {
		add(name)
	}
	return result
}

// GetHostGroups returns all GroupEntry values that contain the given host name.
func (inv *Inventory) GetHostGroups(host string) []*GroupEntry {
	if inv == nil {
		return nil
	}
	var groups []*GroupEntry
	for _, g := range inv.Groups {
		for _, h := range g.Hosts {
			if h == host {
				groups = append(groups, g)
				break
			}
		}
	}
	return groups
}
