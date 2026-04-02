## Why

Bolt's inventory system only supports static YAML files, which forces users to manually maintain host lists. In dynamic infrastructure (cloud environments, auto-scaling groups, container orchestration), hosts change frequently and maintaining static files becomes error-prone and unsustainable. Dynamic inventory sources allow Bolt to discover targets at runtime from external systems, matching how modern infrastructure actually works.

## What Changes

- Add a dynamic inventory loading system that detects inventory source type from the `-i` flag value and delegates to the appropriate provider
- Add a **script/binary plugin** provider that executes an external command and parses its JSON output into Bolt's inventory model
- Add a **built-in AWS EC2 plugin** that discovers instances by tags/filters and auto-populates SSH config from instance metadata (public/private IP, key name)
- Add a **generic HTTP source** provider that fetches inventory JSON from a URL endpoint
- Refactor the existing `-i` flag handling to support type detection: executable files invoke the script plugin, `.yaml`/`.json` extensions use static loading, `ec2://` prefix triggers the EC2 plugin, and `http://`/`https://` prefixes use the HTTP source
- Define a standard JSON schema for dynamic inventory output that maps to the existing `Inventory` struct (hosts, groups, vars)

## Capabilities

### New Capabilities
- `script-inventory-plugin`: Execute external scripts/binaries that output JSON inventory, enabling integration with any system that can produce structured output
- `ec2-inventory-plugin`: Built-in AWS EC2 instance discovery by tags and filters with automatic SSH configuration from instance metadata
- `http-inventory-source`: Fetch inventory from HTTP/HTTPS endpoints, supporting inventory management services and APIs
- `inventory-source-detection`: Automatic detection of inventory source type from the `-i` flag value based on file properties and URI scheme

### Modified Capabilities

## Impact

- `internal/inventory/` - New subpackages for each dynamic source type; `Load()` function evolves into a dispatcher
- `cmd/bolt/main.go` - The `-i` flag handling changes from direct `inventory.Load()` to a source-detection function
- `go.mod` - AWS EC2 SDK dependency already present (`aws-sdk-go-v2/service/ec2`); no new external dependencies needed
- JSON inventory schema becomes a contract for external plugin authors
- Existing static YAML inventories continue to work unchanged (backward compatible)
