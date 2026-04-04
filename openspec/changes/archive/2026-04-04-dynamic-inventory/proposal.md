## Why

Bolt's inventory is limited to static YAML files. This doesn't scale for cloud environments where hosts are ephemeral — instances spin up/down, IPs change, and maintaining static lists becomes a liability. Teams with CMDBs, cloud APIs, or custom tooling need inventory that resolves at runtime from authoritative sources.

## What Changes

- **Plugin architecture**: New `InventoryPlugin` interface for pluggable inventory sources, with a registry and routing logic in `inventory.Load()`
- **Auto-detection**: If `-i` points to an executable file, Bolt runs it as a script plugin instead of parsing it as YAML
- **Plugin dispatch**: YAML files with a `plugin:` key are routed to the named built-in plugin instead of static parsing
- **Script plugin**: Run any executable with `--list`, parse stdout as JSON/YAML in Bolt-native inventory format
- **HTTP plugin**: Fetch inventory from REST APIs with configurable URL, headers, query params, TLS options, and auth (bearer token, basic auth)
- **EC2 plugin**: Discover AWS instances via `DescribeInstances` with tag filters, auto-group by tag key/value pairs
- **`bolt inventory` subcommand**: Dump resolved inventory as JSON for debugging any source type
- **Multiple `-i` flags**: Merge multiple inventory sources with union semantics for hosts/groups and last-wins for scalars
- **Timeouts**: 30s default timeout for script execution and HTTP calls, configurable per-plugin

## Capabilities

### New Capabilities
- `inventory-plugin`: Core plugin interface, registry, routing logic in `inventory.Load()`, and auto-detection of executable vs YAML vs plugin-config files
- `inventory-script`: Script/binary plugin — run executable with `--list`, parse JSON/YAML output, timeout handling
- `inventory-http`: HTTP plugin — fetch inventory from REST endpoints with headers, params, TLS, and auth options
- `inventory-ec2`: EC2 plugin — discover instances via AWS API with tag filters and auto-grouping
- `inventory-cli`: `bolt inventory --list` subcommand for debugging resolved inventory from any source
- `inventory-merge`: Multiple `-i` flag support with merge semantics across sources

### Modified Capabilities

_(none — existing static YAML inventory behavior is preserved as-is)_

## Impact

- **`internal/inventory/`**: New plugin interface, registry, and refactored `Load()` with detection/routing logic. New subpackages for each plugin (`script/`, `http/`, `ec2/`)
- **`cmd/bolt/main.go`**: New `inventory` subcommand, support for multiple `-i` flags, `--inventory-timeout` flag
- **`internal/executor/`**: Accept `[]*Inventory` for merged multi-source inventory (Phase 4)
- **Dependencies**: No new dependencies — reuses existing `aws-sdk-go-v2/service/ec2`, `net/http`, `os/exec` from stdlib
- **Security**: Script plugins execute arbitrary code; must document trust implications. HTTP plugin handles credentials via headers/env vars
