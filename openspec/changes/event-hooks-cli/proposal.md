## Why

Operators running Bolt in CI, cron, or orchestrated pipelines need to know when a run fails — today they must wrap bolt in shell logic and parse exit codes. A minimal hook facility ("run this command when the playbook finishes") unlocks Slack pings, PagerDuty triggers, metric emission, and audit logging with zero playbook changes. A full plugin/callback architecture is the wrong first step: it's speculative and slow to design well. Shipping a CLI flag that spawns a local command with a JSON payload on stdin solves 90% of the use cases with a small, well-bounded surface.

## What Changes

- Add three new CLI flags (repeatable): `--on-failure <cmd>`, `--on-success <cmd>`, `--on-complete <cmd>`. `--on-complete` runs regardless of outcome; `--on-failure` only when the run fails; `--on-success` only when the run succeeds.
- Add `--hook-timeout <duration>` flag (default `30s`) controlling how long each hook may run before SIGTERM.
- Add environment-variable equivalents following the existing `BOLT_*` convention: `BOLT_ON_FAILURE`, `BOLT_ON_SUCCESS`, `BOLT_ON_COMPLETE`, `BOLT_HOOK_TIMEOUT`. Flags take precedence over env vars; env vars accept comma-separated commands for repetition.
- At the end of a run, spawn each matching command via `/bin/sh -c <cmd>` on the control host (where bolt is running, not on targets), feeding a JSON payload on stdin.
- JSON payload shape: `{run_id, status, playbook, started_at, ended_at, duration_ms, failed_task_count, changed_task_count, ok_task_count, hosts: [{name, status, failed_tasks, ok_task_count, changed_task_count, duration_ms}]}`.
- Hooks run sequentially in registration order: all `--on-failure` (or `--on-success`), then all `--on-complete`.
- Hook stdout/stderr is captured; shown in verbose mode; hook exit code is recorded but does NOT change the playbook run's exit code. Hook failures are logged as warnings.
- Scope explicitly excludes: webhooks (users wrap `curl`), per-task hooks, plugin architecture, callback registry. Those are v2.

## Capabilities

### New Capabilities
- `event-hooks-cli`: End-of-run command invocation via CLI flags, feeding a JSON summary payload over stdin, with timeout enforcement and captured output — enabling notifications and alerting without modifying playbooks.

### Modified Capabilities

None.

## Impact

- New package: `internal/hooks/` (runner, payload builder, timeout enforcement)
- `cmd/bolt/main.go` gains four new flags on the run/execute command and an env-var resolver
- Executor / output pipeline must expose an end-of-run summary struct that the hook package consumes
- No new external dependencies
- Documentation: README section, `docs/hooks.md`, `llms.txt` entry, example CI snippet (Slack + curl)
- Testing: unit tests for payload construction, timeout handling, flag+env precedence; integration test spawning a tiny script and verifying JSON payload
