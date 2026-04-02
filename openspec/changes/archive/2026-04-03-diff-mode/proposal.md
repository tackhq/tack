## Why

The `--verbose` flag currently controls both "more logging" and "show file content diffs in plan output." Users expect verbose to mean more logging, not a completely different diff display. A dedicated `--diff` flag would give users explicit control over seeing exactly what file changes Bolt will make — critical for auditing, code review of infrastructure changes, and building confidence before applying to production.

## What Changes

- Add a `--diff` CLI flag that shows file content diffs in plan output, independent of `--verbose`
- Add diff file path headers (`---`/`+++`) so users can tell which file a diff belongs to
- Handle new file creation (show all content as additions) and file deletion (show all as removals) in diff display
- Replace the naive two-pointer diff algorithm with a proper unified diff (Myers algorithm via `go-difflib`)
- Add context-window limiting to suppress long unchanged sections (±3 lines around changes)
- Detect binary files and large files, falling back to checksum-only display
- Gate the expensive remote `cat` (used to fetch existing file content) behind the diff/verbose flag
- Enrich JSON output with checksum and optional content fields for CI tooling

## Capabilities

### New Capabilities
- `diff-output`: Dedicated `--diff` flag, diff rendering with headers/context/binary detection, and JSON diff data

### Modified Capabilities
- `json-output`: Add checksum and optional content fields to `plan_task` JSON events
- `cli-ux-improvements`: Add `--diff` flag to CLI, decouple diff display from `--verbose`

## Impact

- **CLI**: New `--diff` persistent flag on root command
- **Output layer**: `internal/output/output.go` — new `diff` field on `Output` struct, enhanced `DisplayPlan`, improved `unifiedDiff`
- **JSON output**: `internal/output/json.go` — enriched `plan_task` events
- **Module utilities**: `internal/module/fileutil.go` — gate remote `cat` behind diff flag, binary/size checks
- **Dependencies**: Promote `github.com/pmezard/go-difflib` from indirect to direct dependency
- **No breaking changes** — existing `--verbose` behavior preserved
