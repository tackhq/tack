package inventory

// MergeInventories merges multiple inventories into one. Sources are applied
// in order: later sources override scalar values and append to host lists.
func MergeInventories(inventories []*Inventory) *Inventory {
	if len(inventories) == 0 {
		return &Inventory{}
	}
	if len(inventories) == 1 {
		return inventories[0]
	}

	merged := &Inventory{
		Hosts:  make(map[string]*HostEntry),
		Groups: make(map[string]*GroupEntry),
	}

	for _, inv := range inventories {
		if inv == nil {
			continue
		}

		// Merge hosts: later source overrides scalar fields
		for name, host := range inv.Hosts {
			existing, ok := merged.Hosts[name]
			if !ok {
				// Deep copy
				entry := &HostEntry{}
				if host.SSH != nil {
					sshCopy := *host.SSH
					entry.SSH = &sshCopy
				}
				if host.Vars != nil {
					entry.Vars = copyVars(host.Vars)
				}
				merged.Hosts[name] = entry
				continue
			}
			// Override SSH if provided
			if host.SSH != nil {
				sshCopy := *host.SSH
				existing.SSH = &sshCopy
			}
			// Deep-merge vars
			if host.Vars != nil {
				if existing.Vars == nil {
					existing.Vars = make(map[string]any)
				}
				deepMergeVars(existing.Vars, host.Vars)
			}
		}

		// Merge groups: union hosts, deep-merge vars, later wins on connection/SSH/SSM
		for name, group := range inv.Groups {
			existing, ok := merged.Groups[name]
			if !ok {
				// Deep copy
				entry := &GroupEntry{
					Connection: group.Connection,
					Hosts:      append([]string{}, group.Hosts...),
				}
				if group.SSH != nil {
					sshCopy := *group.SSH
					entry.SSH = &sshCopy
				}
				if group.SSM != nil {
					ssmCopy := *group.SSM
					entry.SSM = &ssmCopy
				}
				if group.Vars != nil {
					entry.Vars = copyVars(group.Vars)
				}
				merged.Groups[name] = entry
				continue
			}

			// Override connection if set
			if group.Connection != "" {
				existing.Connection = group.Connection
			}

			// Override SSH/SSM if set
			if group.SSH != nil {
				sshCopy := *group.SSH
				existing.SSH = &sshCopy
			}
			if group.SSM != nil {
				ssmCopy := *group.SSM
				existing.SSM = &ssmCopy
			}

			// Union hosts (deduplicated)
			seen := make(map[string]bool, len(existing.Hosts))
			for _, h := range existing.Hosts {
				seen[h] = true
			}
			for _, h := range group.Hosts {
				if !seen[h] {
					existing.Hosts = append(existing.Hosts, h)
					seen[h] = true
				}
			}

			// Deep-merge vars
			if group.Vars != nil {
				if existing.Vars == nil {
					existing.Vars = make(map[string]any)
				}
				deepMergeVars(existing.Vars, group.Vars)
			}
		}
	}

	return merged
}

// deepMergeVars merges src into dst. Later values win on conflicts.
func deepMergeVars(dst, src map[string]any) {
	for k, v := range src {
		dst[k] = v
	}
}

func copyVars(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
