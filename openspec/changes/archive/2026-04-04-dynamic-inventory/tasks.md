## 1. Plugin Framework

- [x] 1.1 Create `internal/inventory/plugin.go` with `Plugin` interface (`Name()`, `Load(ctx, config)`) and registry (Register, Get functions)
- [x] 1.2 Refactor `inventory.Load()` to route: executable → script plugin, `plugin:` key → named plugin, else → static YAML parse
- [x] 1.3 Add `--inventory-timeout` CLI flag (default 30s) and pass context with timeout to plugin Load calls
- [x] 1.4 Write tests for routing logic: executable detection, plugin key dispatch, static fallback

## 2. Script Plugin

- [x] 2.1 Create `internal/inventory/script/script.go` implementing Plugin interface — exec with `--list`, capture stdout/stderr
- [x] 2.2 Implement output format detection (JSON vs YAML based on first non-whitespace char)
- [x] 2.3 Handle error cases: non-zero exit (include stderr), empty output, timeout (kill process)
- [x] 2.4 Register script plugin and wire into routing (executable auto-detection path)
- [x] 2.5 Write tests with mock scripts (JSON output, YAML output, failure, timeout, empty output)

## 3. HTTP Plugin

- [x] 3.1 Create `internal/inventory/http/http.go` implementing Plugin interface — GET request, parse response
- [x] 3.2 Implement config parsing: url (required), headers, params, timeout, auth (basic), tls options
- [x] 3.3 Implement TLS configuration: ca_cert, client_cert/key, insecure_skip_verify
- [x] 3.4 Implement `{{ env.VAR }}` interpolation in string config values
- [x] 3.5 Handle error cases: non-2xx status (include truncated body), network errors, missing url
- [x] 3.6 Register HTTP plugin in registry
- [x] 3.7 Write tests with httptest server (success, auth, TLS, errors, env interpolation)

## 4. EC2 Plugin

- [x] 4.1 Create `internal/inventory/ec2/ec2.go` implementing Plugin interface — DescribeInstances with filters
- [x] 4.2 Implement config parsing: regions (required), filters, group_by, host_key (default: private_ip)
- [x] 4.3 Implement multi-region querying and result merging
- [x] 4.4 Implement auto-grouping by tags (`tag_{key}_{value}` naming, deduplicated host lists)
- [x] 4.5 Map instance tags to host vars (lowercase keys, hyphens to underscores)
- [x] 4.6 Set connection defaults based on host_key (instance_id → ssm, IP → ssh)
- [x] 4.7 Register EC2 plugin in registry
- [x] 4.8 Write tests with mocked EC2 API (discovery, grouping, multi-region, empty results, errors)

## 5. Inventory CLI Subcommand

- [x] 5.1 Add `bolt inventory` command with `--list` flag to `cmd/bolt/main.go`
- [x] 5.2 Implement `--host <name>` flag for single-host detail (merged vars, group memberships)
- [x] 5.3 JSON output formatting for both list and host modes
- [x] 5.4 Error handling: no `-i` provided, unknown host name
- [x] 5.5 Write tests for inventory subcommand (list, host detail, error cases)

## 6. Multiple Inventory Sources (Merge)

- [x] 6.1 Update CLI to accept multiple `-i` flags (collect into string slice)
- [x] 6.2 Implement `MergeInventories(inventories []*Inventory) *Inventory` — host union with later-wins scalars, group union with deep-merged vars
- [x] 6.3 Wire merge into executor: load each `-i` source, merge, pass single `*Inventory` to executor
- [x] 6.4 Write tests for merge semantics (host override, group host union, var deep-merge, ordering)

## 7. Documentation & Integration

- [x] 7.1 Add dynamic inventory examples to `examples/` (script, HTTP config, EC2 config)
- [x] 7.2 Update README with dynamic inventory section
- [x] 7.3 Update `llms.txt` and docs with new plugin system and CLI subcommand
- [x] 7.4 Mark dynamic inventory as implemented in ROADMAP.md
