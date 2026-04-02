## Context

Bolt currently loads inventory exclusively through `inventory.Load()`, which reads a YAML file from disk and unmarshals it into an `Inventory` struct containing `Hosts` and `Groups` maps. The `-i` flag in `cmd/bolt/main.go` passes a file path directly to this function. There is no abstraction for different inventory sources.

The existing `Inventory` struct and its methods (`ExpandGroup`, `GetHost`, `AllHosts`, `GetHostGroups`) are well-designed and should remain the canonical in-memory representation. Dynamic sources need only produce this same struct.

AWS SDK v2 dependencies (including `service/ec2`) are already in `go.mod`, so the EC2 plugin requires no new external dependencies.

## Goals / Non-Goals

**Goals:**
- Support runtime host discovery from external scripts, AWS EC2, and HTTP endpoints
- Auto-detect inventory source type from the `-i` flag value without additional flags
- Define a JSON schema that external script plugins must produce
- Maintain full backward compatibility with existing static YAML inventories
- EC2 plugin auto-populates SSH connection config from instance metadata

**Non-Goals:**
- Caching or persistent storage of dynamic inventory results (always fresh on each run)
- Inventory source composition (combining multiple sources in one run)
- Custom plugin discovery paths or plugin registries
- GCE, Azure, or other cloud provider plugins (EC2 only for now)
- Watch/streaming inventory updates during a playbook run

## Decisions

### 1. Source detection via URI scheme and file properties

The `-i` flag value determines the source type:

| Pattern | Source Type |
|---|---|
| `ec2://region?tag=value&...` | EC2 plugin |
| `http://...` or `https://...` | HTTP source |
| Executable file (os.Stat + mode check) | Script plugin |
| Everything else (`.yaml`, `.yml`, `.json`) | Static file |

**Rationale**: This avoids adding a separate `--inventory-type` flag. The patterns are unambiguous -- an executable file is clearly a script, and URI schemes clearly indicate remote sources. This matches Ansible's convention where inventory scripts are detected by being executable.

**Alternative considered**: A `--inventory-type` flag. Rejected because it adds UX friction and the detection heuristics are reliable.

### 2. Standard JSON inventory format

All dynamic sources produce JSON conforming to this structure:

```json
{
  "hosts": {
    "web-1": {
      "ssh": { "host": "10.0.1.5", "user": "ubuntu", "port": 22, "key": "~/.ssh/id_rsa" },
      "vars": { "role": "web" }
    }
  },
  "groups": {
    "webservers": {
      "connection": "ssh",
      "ssh": { "user": "ubuntu" },
      "hosts": ["web-1", "web-2"],
      "vars": { "env": "production" }
    }
  }
}
```

This mirrors the existing YAML inventory format exactly, making deserialization trivial (unmarshal into `Inventory` struct).

**Rationale**: Using the same structure as static YAML means no translation layer. External script authors can reference the YAML inventory docs to understand the expected output.

### 3. InventorySource interface

```go
type InventorySource interface {
    Load(ctx context.Context) (*Inventory, error)
}
```

Each source type implements this interface. A top-level `LoadFromSource(ctx, source string) (*Inventory, error)` function detects the source type and delegates.

**Rationale**: A single interface keeps the abstraction minimal. Context parameter enables timeout/cancellation for network-based sources (HTTP, EC2). The detection function is the only new public API surface.

### 4. EC2 plugin uses URI-encoded parameters

Format: `ec2://us-east-1?tag:Environment=production&tag:Role=web&vpc-id=vpc-123`

- Path segment is the AWS region
- Query parameters map to EC2 DescribeInstances filters
- `tag:Key=Value` parameters become tag filters
- Other parameters become standard EC2 filters

**Rationale**: URI encoding is compact, unambiguous, and parseable with `net/url`. It avoids needing a separate config file for EC2 parameters.

### 5. Package structure

```
internal/inventory/
  inventory.go          (existing -- add LoadFromSource dispatcher)
  source.go             (InventorySource interface + detection logic)
  script.go             (script/binary plugin)
  ec2.go                (EC2 plugin)
  http.go               (HTTP source)
```

All source implementations live in the same `inventory` package rather than subpackages.

**Rationale**: The sources are tightly coupled to the `Inventory` type. Subpackages would create import cycles or require extracting types to a separate package. The total added code is modest (each source is ~100-150 lines).

### 6. Script plugin execution model

- Execute the script with `exec.CommandContext` (inherits env vars)
- Capture stdout as the JSON inventory
- Stderr is passed through to bolt's stderr for debugging
- Non-zero exit code is an error
- Timeout controlled by context (inherits bolt's global timeout)

**Rationale**: This matches Ansible's dynamic inventory script convention. Passing through stderr enables script authors to emit debug info without polluting the inventory output.

## Risks / Trade-offs

- **[Script plugin security]** Executing arbitrary binaries is inherently risky. Mitigation: The user explicitly specifies the script path, same trust model as running any playbook. Document that scripts should be reviewed before use.
- **[EC2 API latency]** DescribeInstances can be slow with large fleets. Mitigation: No caching in v1 -- users accept the latency for correctness. Future enhancement could add `--cache-inventory` flag.
- **[HTTP source reliability]** Network failures block playbook execution. Mitigation: Standard HTTP timeouts via context; clear error messages indicating the source URL that failed.
- **[JSON-only dynamic format]** Script plugins must output JSON, not YAML. Mitigation: JSON is easier to produce from any language. YAML output support could be added later by sniffing content type.
