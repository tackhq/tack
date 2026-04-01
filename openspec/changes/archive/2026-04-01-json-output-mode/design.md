## Context

Bolt's output system is a set of Print* functions in `internal/output/output.go` that write ANSI-colored text directly to a writer (typically os.Stdout). There is no abstraction layer — formatting is baked into each function. Adding JSON output requires either duplicating every function or introducing an output strategy pattern.

## Goals / Non-Goals

**Goals:**
- Emit structured JSON events for all execution phases
- Use NDJSON format (one JSON object per line) for streaming compatibility
- Include timestamps on each event for log correlation
- Auto-approve in JSON mode (no interactive prompts on stdout)
- Clean separation: JSON to stdout, errors to stderr

**Non-Goals:**
- JSON input (reading playbooks from JSON)
- GraphQL or REST API output
- Custom output format plugins
- JSON output for `bolt validate` or `bolt modules` (future enhancement)

## Decisions

### 1. NDJSON (newline-delimited JSON) format

Each event is a self-contained JSON object on its own line. This enables streaming parsers to process output as it arrives, rather than waiting for a complete JSON array.

**Alternative considered:** Single JSON document with nested arrays. Rejected because it requires buffering all output until completion, preventing real-time streaming in pipelines.

### 2. Output strategy interface

Introduce an `Emitter` interface with methods matching the current Print* functions. Two implementations: `TextEmitter` (current behavior) and `JSONEmitter` (new). The executor and output package use the interface, selecting the implementation based on `--output`.

**Alternative considered:** Conditional checks (`if jsonMode`) in every Print function. Rejected — violates Open/Closed principle and makes the code harder to maintain.

### 3. Event types with consistent schema

Each JSON event has: `type` (string), `timestamp` (ISO 8601), and type-specific fields. Event types:
- `playbook_start`: `{playbook: path}`
- `play_start`: `{play: name, hosts: [...]}`
- `plan_task`: `{host, task, module, action, params}` (during plan phase)
- `task_start`: `{host, task, module}`
- `task_result`: `{host, task, module, status, changed, message, data}`
- `host_recap`: `{host, ok, changed, failed, skipped, duration}`
- `playbook_recap`: `{ok, changed, failed, skipped, duration, success}`

### 4. JSON mode implies --auto-approve

Interactive prompts write to stdout and read from stdin. In JSON mode, stdout is reserved for structured data. Therefore JSON mode automatically enables `--auto-approve` to skip prompts.

## Risks / Trade-offs

- **[Risk] JSON schema versioning** — Changing event fields breaks consumers. → Mitigation: Document the schema; add a `version` field to playbook_start event for future evolution.
- **[Trade-off] No plan output in JSON mode** — Plan phase could emit events but adds complexity. → Include plan events (plan_task) so CI can preview without applying.
- **[Trade-off] Emitter interface adds abstraction** — More code than the current direct approach. → Worth it for maintainability and testability.
