## Context

Bolt currently loads inventory from a single static YAML file via `inventory.Load(path)` in `internal/inventory/inventory.go`. The function reads the file, unmarshals it into an `Inventory` struct with `Hosts` and `Groups` maps, and returns it. The executor consumes this via `exec.Inventory` field — expanding groups, injecting vars, and merging SSH/SSM config.

Cloud environments need runtime discovery. AWS fleets change constantly, CMDBs are the source of truth, and teams maintain custom scripts that generate host lists. The current static-only approach forces users to regenerate YAML files externally before each run.

## Goals / Non-Goals

**Goals:**
- Pluggable inventory sources that return the same `*Inventory` struct
- Transparent to the executor — dynamic sources produce identical data as static YAML
- Auto-detection of executable files for Ansible-like UX
- Built-in plugins for common sources (EC2, HTTP)
- Debuggable — users can inspect resolved inventory before running playbooks
- Multiple sources can be merged into a unified inventory

**Non-Goals:**
- External plugin protocol (gRPC, hashicorp/go-plugin) — script plugin covers external binaries
- Ansible inventory format compatibility — Bolt-native format only
- Mid-run inventory refresh — resolve once at startup
- Inventory caching/TTL — users can implement caching in their scripts
- Cloud providers beyond AWS (GCP, Azure) — addressable via script plugin

## Decisions

### 1. Plugin interface design

```go
type Plugin interface {
    Name() string
    Load(ctx context.Context, config map[string]any) (*Inventory, error)
}
```

Plugins return `*Inventory` directly — the same struct static YAML produces. This means zero changes to the executor for Phases 1-3. The `config` parameter receives the parsed YAML content (minus the `plugin:` key) for plugin-config files.

**Alternative considered:** Returning a generic `[]Host` list and building `Inventory` in the caller. Rejected because plugins need to define groups, vars, and connection settings — the full `Inventory` struct is the right abstraction.

### 2. Detection and routing in Load()

Refactor `inventory.Load(path)` into a three-step router:

1. `os.Stat(path)` — if executable bit set → script plugin
2. Read file, YAML unmarshal into `map[string]any` — if `plugin` key exists → dispatch to named plugin
3. Otherwise → current static YAML parse (existing behavior, unchanged)

**Alternative considered:** URI scheme prefix (`script://`, `ec2://`). Rejected in favor of auto-detection for Ansible compatibility. URI schemes can be added later as a non-breaking enhancement.

### 3. Script plugin protocol

Run executable with `--list` argument. Parse stdout as JSON (if first non-whitespace char is `{`) or YAML otherwise. Stderr is captured and included in error messages on failure. Non-zero exit code is an error.

Child process inherits parent environment (Go default) — scripts access credentials via env vars. Timeout via context (30s default).

**Alternative considered:** Also supporting `--host <name>` for per-host var lookup. Deferred — Bolt always loads full inventory upfront, so `--list` is sufficient.

### 4. HTTP plugin configuration

```yaml
plugin: http
url: https://cmdb.internal/api/inventory
headers:
  Authorization: "Bearer {{ env.CMDB_TOKEN }}"
params:
  env: production
timeout: 10
tls:
  ca_cert: /etc/ssl/custom-ca.pem
  client_cert: /etc/ssl/client.pem
  client_key: /etc/ssl/client-key.pem
  insecure_skip_verify: false
auth:
  basic:
    username: "{{ env.CMDB_USER }}"
    password: "{{ env.CMDB_PASS }}"
```

Variable interpolation (`{{ env.* }}`) in config values uses the existing Bolt variable system. Response body parsed as JSON/YAML in Bolt-native format.

### 5. EC2 plugin design

```yaml
plugin: ec2
regions: [us-east-1, us-west-2]
filters:
  tag:env: production
  tag:service: api
group_by: [tag:role, tag:env]
host_key: private_ip  # or instance_id, public_ip
```

Uses `ec2.DescribeInstances` with tag filters. Each instance becomes a host entry. `group_by` creates groups like `tag_role_worker`, `tag_env_production`. All tags are added as host vars. The `host_key` setting controls what value is used as the host identifier.

Reuses existing `aws-sdk-go-v2/service/ec2` dependency. AWS credentials from standard SDK chain (env, shared config, IAM roles).

### 6. Inventory merge strategy (Phase 4)

When multiple `-i` flags are provided, each source produces an `*Inventory`. Merge in order:
- **Hosts**: Union by name. Later sources override `Vars` and `SSH` for same host name.
- **Groups**: Union by name. Host lists are concatenated (deduplicated). Vars deep-merged with later sources winning on conflicts.
- **Connection settings**: Later source wins.

The executor receives a single merged `*Inventory` — no interface changes needed.

### 7. File organization

```
internal/inventory/
├── inventory.go          # Existing — add routing logic to Load()
├── plugin.go             # Plugin interface + registry
├── script/
│   └── script.go         # Script plugin implementation
├── http/
│   └── http.go           # HTTP plugin implementation
└── ec2/
    └── ec2.go            # EC2 plugin implementation
```

Each plugin in its own subpackage, consistent with the existing connector and module patterns.

## Risks / Trade-offs

**[Security: script execution]** → Running `-i ./script` executes arbitrary code. Mitigation: document trust implications clearly. This is identical to Ansible's model and to running any CLI tool.

**[Reliability: external source unavailability]** → HTTP endpoint or AWS API down blocks playbook execution. Mitigation: configurable timeouts with clear error messages. No retry/fallback — fail fast so the user can diagnose.

**[Complexity: variable interpolation in plugin config]** → Plugin config files need `{{ env.* }}` expansion before being passed to plugins. Mitigation: reuse existing `InterpolateString` from the playbook package. Only `env.*` context available at inventory-load time (no facts/play vars yet).

**[Scope creep: EC2 grouping flexibility]** → Users will want complex grouping logic (composite keys, regex filters). Mitigation: start with simple `tag:<key>` grouping. Complex needs are served by writing a custom script plugin.

## Open Questions

- Should `bolt inventory --list` support `--format yaml|json` or just JSON?
- Should EC2 plugin support assume-role for cross-account discovery, or rely on the standard AWS credential chain?
