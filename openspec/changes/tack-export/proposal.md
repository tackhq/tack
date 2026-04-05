## Why

Three unmet needs keep appearing: (1) **compliance audits** — security reviewers want to see the exact commands that will run on production, not read YAML abstractions; (2) **air-gapped / restricted environments** — some hosts cannot run tack directly but can receive and execute a shell script; (3) **debugging** — "why did this task do X?" is easiest to answer by reading the literal shell command. A `tack export` subcommand compiles a playbook down to a standalone bash script per host, with all variables/templates resolved, producing a human-readable artifact that matches what each module would send through the local connector.

## What Changes

- Add a new CLI subcommand `tack export <playbook>` with flags: `--host <name>` (single host, default behavior when only one inventory host matches), `--all-hosts` (emit one script per host in inventory), `--output <path>` (file or directory depending on mode; stdout when unset in single-host mode), `--no-facts` (skip fact gathering; leave fact references as TODO comments), `--check-only` (validate + list unsupported constructs, do not write files), `-e key=value` and `--extra-vars` (same semantics as `run`), `--tags`/`--skip-tags` (pre-filter tasks during export).
- The emitted script: starts with `#!/usr/bin/env bash` + `set -euo pipefail`, a banner (tack version, export timestamp UTC, playbook path, host name, frozen-facts marker), counters `TACK_CHANGED=0` / `TACK_FAILED=0`, and a trap that reports the failing task on exit.
- Each task emits a fenced block: `# === TASK: <name> === (tags: a,b)`, followed by the shell commands, followed by a counter bump.
- Facts frozen at export time by gathering against the target via the selected connector (local by default; `--connection` flag supported). Frozen facts appear as comments in the script header for auditability.
- Templates (`template:`), interpolations (`{{ var }}`), loops (`loop:` on simple static lists), `when:` conditions, tags, and shorthand are all resolved at export time.
- Unsupported constructs are emitted as `# UNSUPPORTED: <reason>` block comments instead of being silently dropped. Supported set: `command`, `apt`, `brew`, `yum`, `file`, `copy`, `template`, `lineinfile`, `blockinfile`, `systemd`, `user`, `group`, `cron` (when merged), `git` (when merged), `assert` (when merged). Unsupported: handlers, `register` with downstream runtime use, async, `wait_for`, `delegate_to`, `include_tasks` with dynamic/loop inclusion, `block`/`rescue`/`always` (dropped entirely in v1 — emitted as `# UNSUPPORTED` with exit code non-zero in `--check-only`).
- Output is deterministic: stable ordering, sorted maps, pinned timestamps to the export invocation, no randomness.
- `--check-only` mode prints a summary of supported/unsupported tasks and exits non-zero if any unsupported construct is encountered.

## Capabilities

### New Capabilities
- `tack-export`: Compile a playbook into a standalone bash script per host, resolving variables, templates, tags, and conditions at export time. Supports single-host or all-hosts mode, fact freezing, `--check-only` validation, and deterministic output suitable for audit and air-gapped execution.

### Modified Capabilities

None. (This adds a new CLI subcommand and a new module capability — the "emit shell payload" operation — but does not modify existing module behavior.)

## Impact

- New package: `internal/export/` (compiler, script template, banner/counter helpers)
- New CLI subcommand in `cmd/tack/main.go`: `export`
- **Module interface extension**: each supported module gains an optional `Emit(params map[string]any, pctx *PlayContext) (*EmitResult, error)` method returning shell script text + supported/unsupported flag. Modules that don't implement it are reported as unsupported. This is additive (interface-wise: a new optional `Emitter` interface alongside `Checker`).
- Fact gathering integration: reuse `pkg/facts` output; freeze into an in-memory map and render as comments.
- Documentation: new `docs/export.md`, README section, `llms.txt` entries, CI example for generating and diff-reviewing audit scripts.
- Testing: unit tests per module's Emit implementation; golden-file tests for representative playbook → script conversions; `--check-only` behavior tests.
- ROADMAP.md update — last P2 item marked done.
