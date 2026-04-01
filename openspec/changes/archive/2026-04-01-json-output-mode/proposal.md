## Why

Bolt has no machine-readable output format. CI/CD pipelines, monitoring systems, and automation wrappers cannot programmatically parse execution results. Every modern infrastructure tool (Terraform, kubectl, gh) supports `--output json`. Without this, Bolt is limited to interactive terminal use.

## What Changes

- Add `--output` flag accepting `text` (default) or `json`
- JSON mode emits newline-delimited JSON (NDJSON) — one JSON object per event
- Events include: playbook_start, play_start, task_result, host_recap, playbook_recap
- JSON mode implies `--auto-approve` (no interactive prompts)
- JSON output goes to stdout; errors go to stderr
- Existing text output remains the default and is unchanged

## Capabilities

### New Capabilities
- `json-output`: Structured JSON output mode for CI/CD integration and programmatic consumption

### Modified Capabilities

_None — existing text output is unchanged._

## Impact

- **Modified code**: `internal/output/output.go` — add JSON emitter alongside existing text output
- **Modified code**: `cmd/bolt/main.go` — add `--output` flag, wire to output package
- **Modified code**: `internal/executor/executor.go` — pass output mode, skip approval prompt in JSON mode
- **No dependency changes** — uses `encoding/json` from standard library
- **No breaking changes** — default output mode unchanged
