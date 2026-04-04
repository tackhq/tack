package inventory

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeInventories_Empty(t *testing.T) {
	result := MergeInventories(nil)
	assert.NotNil(t, result)
	assert.Empty(t, result.Hosts)
}

func TestMergeInventories_Single(t *testing.T) {
	inv := &Inventory{
		Hosts: map[string]*HostEntry{
			"web1": {Vars: map[string]any{"env": "prod"}},
		},
	}
	result := MergeInventories([]*Inventory{inv})
	assert.Equal(t, inv, result)
}

func TestMergeInventories_HostOverride(t *testing.T) {
	inv1 := &Inventory{
		Hosts: map[string]*HostEntry{
			"web1": {Vars: map[string]any{"region": "us-east-1", "env": "staging"}},
		},
	}
	inv2 := &Inventory{
		Hosts: map[string]*HostEntry{
			"web1": {Vars: map[string]any{"region": "eu-west-1"}},
		},
	}

	result := MergeInventories([]*Inventory{inv1, inv2})
	require.Contains(t, result.Hosts, "web1")
	// Later source wins on conflicts
	assert.Equal(t, "eu-west-1", result.Hosts["web1"].Vars["region"])
	// Non-conflicting keys preserved
	assert.Equal(t, "staging", result.Hosts["web1"].Vars["env"])
}

func TestMergeInventories_UniqueHosts(t *testing.T) {
	inv1 := &Inventory{
		Hosts: map[string]*HostEntry{
			"web1": {Vars: map[string]any{"from": "inv1"}},
		},
	}
	inv2 := &Inventory{
		Hosts: map[string]*HostEntry{
			"db1": {Vars: map[string]any{"from": "inv2"}},
		},
	}

	result := MergeInventories([]*Inventory{inv1, inv2})
	assert.Contains(t, result.Hosts, "web1")
	assert.Contains(t, result.Hosts, "db1")
}

func TestMergeInventories_GroupHostsUnion(t *testing.T) {
	inv1 := &Inventory{
		Groups: map[string]*GroupEntry{
			"web": {Hosts: []string{"web1"}, Vars: map[string]any{"port": 8080}},
		},
	}
	inv2 := &Inventory{
		Groups: map[string]*GroupEntry{
			"web": {Hosts: []string{"web2"}, Vars: map[string]any{"port": 9090, "region": "us-east-1"}},
		},
	}

	result := MergeInventories([]*Inventory{inv1, inv2})
	require.Contains(t, result.Groups, "web")
	g := result.Groups["web"]

	hosts := append([]string{}, g.Hosts...)
	sort.Strings(hosts)
	assert.Equal(t, []string{"web1", "web2"}, hosts)

	// Vars deep-merged, later wins
	assert.Equal(t, 9090, g.Vars["port"])
	assert.Equal(t, "us-east-1", g.Vars["region"])
}

func TestMergeInventories_GroupHostsDedup(t *testing.T) {
	inv1 := &Inventory{
		Groups: map[string]*GroupEntry{
			"web": {Hosts: []string{"web1", "web2"}},
		},
	}
	inv2 := &Inventory{
		Groups: map[string]*GroupEntry{
			"web": {Hosts: []string{"web1", "web3"}},
		},
	}

	result := MergeInventories([]*Inventory{inv1, inv2})
	hosts := result.Groups["web"].Hosts
	sort.Strings(hosts)
	assert.Equal(t, []string{"web1", "web2", "web3"}, hosts)
}

func TestMergeInventories_ThreeSourcesOrdering(t *testing.T) {
	inv1 := &Inventory{
		Hosts: map[string]*HostEntry{
			"h1": {Vars: map[string]any{"val": "first"}},
		},
	}
	inv2 := &Inventory{
		Hosts: map[string]*HostEntry{
			"h1": {Vars: map[string]any{"val": "second"}},
		},
	}
	inv3 := &Inventory{
		Hosts: map[string]*HostEntry{
			"h1": {Vars: map[string]any{"val": "third"}},
		},
	}

	result := MergeInventories([]*Inventory{inv1, inv2, inv3})
	assert.Equal(t, "third", result.Hosts["h1"].Vars["val"])
}

func TestMergeInventories_GroupConnectionOverride(t *testing.T) {
	inv1 := &Inventory{
		Groups: map[string]*GroupEntry{
			"app": {Connection: "ssh", Hosts: []string{"h1"}},
		},
	}
	inv2 := &Inventory{
		Groups: map[string]*GroupEntry{
			"app": {Connection: "ssm", Hosts: []string{"h1"}},
		},
	}

	result := MergeInventories([]*Inventory{inv1, inv2})
	assert.Equal(t, "ssm", result.Groups["app"].Connection)
}
