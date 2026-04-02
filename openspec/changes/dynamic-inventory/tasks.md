## 1. Core Interface and Source Detection

- [ ] 1.1 Define `InventorySource` interface in `internal/inventory/source.go` with `Load(ctx context.Context) (*Inventory, error)` method
- [ ] 1.2 Implement `LoadFromSource(ctx context.Context, source string) (*Inventory, error)` dispatcher that detects source type by URI scheme and file properties
- [ ] 1.3 Add static file source wrapper (`staticSource`) that wraps the existing `Load()` function to satisfy the `InventorySource` interface, including JSON file support
- [ ] 1.4 Write tests for source detection logic: ec2:// prefix, http/https prefix, executable file, static YAML/JSON, non-existent file error

## 2. Script Inventory Plugin

- [ ] 2.1 Implement `scriptSource` in `internal/inventory/script.go` that executes the script with `exec.CommandContext`, captures stdout as JSON, and passes stderr through
- [ ] 2.2 Handle error cases: non-zero exit code, invalid JSON output, context cancellation
- [ ] 2.3 Write tests for script plugin using a test helper script (valid output, invalid JSON, non-zero exit, environment inheritance)

## 3. HTTP Inventory Source

- [ ] 3.1 Implement `httpSource` in `internal/inventory/http.go` with GET request, 30s default timeout, Accept header, and redirect limit
- [ ] 3.2 Add `BOLT_INVENTORY_TOKEN` bearer token support via Authorization header
- [ ] 3.3 Handle error cases: non-2xx status, invalid JSON response, context cancellation
- [ ] 3.4 Write tests for HTTP source using `httptest.NewServer` (valid response, auth token, error status, invalid JSON)

## 4. EC2 Inventory Plugin

- [ ] 4.1 Implement `ec2Source` in `internal/inventory/ec2.go` that parses the `ec2://region?filters` URI format
- [ ] 4.2 Call DescribeInstances with parsed tag filters and standard filters, filtering to running instances only
- [ ] 4.3 Auto-populate SSH config: public IP (fallback to private), key path from KeyName, default user based on platform
- [ ] 4.4 Populate host vars from instance tags plus instance_id and instance_type
- [ ] 4.5 Auto-create groups from tag values (`tag_<Key>_<Value>`)
- [ ] 4.6 Write tests using interface-based EC2 API mock (matching existing SSM mock pattern)

## 5. CLI Integration

- [ ] 5.1 Update `cmd/bolt/main.go` to replace direct `inventory.Load()` call with `inventory.LoadFromSource()`, passing context
- [ ] 5.2 Verify backward compatibility: existing static YAML inventory files produce identical results

## 6. Documentation

- [ ] 6.1 Add inventory JSON schema example to a doc comment or example file for external script authors
- [ ] 6.2 Update CLI help text for `-i` flag to describe supported source types (file, executable, ec2://, http://)
