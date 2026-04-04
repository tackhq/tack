## 1. Package Scaffold

- [ ] 1.1 Create `internal/hooks/` package with `runner.go`, `payload.go`, `config.go`
- [ ] 1.2 Define `Config` struct: OnFailure []string, OnSuccess []string, OnComplete []string, Timeout time.Duration
- [ ] 1.3 Define `Payload` struct matching the documented JSON shape (tagged with json tags)
- [ ] 1.4 Define `HostSummary` and `FailedTask` sub-structs

## 2. Run Summary Integration

- [ ] 2.1 Locate the existing end-of-run summary accumulator in the executor/output pipeline
- [ ] 2.2 Add an exported `RunSummary` type (or reuse existing) that exposes: playbook path, started/ended timestamps, task counts, per-host counts + status + failed tasks
- [ ] 2.3 Ensure per-host failed_tasks honor no_log/vault redaction (reuse existing redaction helpers, write test)
- [ ] 2.4 Track unreachable hosts as a distinct status and fold into `status: failed` at run level

## 3. Payload Builder

- [ ] 3.1 Implement `BuildPayload(summary RunSummary, runID string) *Payload` that translates the internal summary to the stable payload shape
- [ ] 3.2 Compute run-level status: `failed` if any host failed or unreachable, else `success`
- [ ] 3.3 Format durations as int64 milliseconds; timestamps as RFC3339
- [ ] 3.4 Hard-code `schema_version: 1`

## 4. CLI Flags & Env Resolution

- [ ] 4.1 Add `--on-failure`, `--on-success`, `--on-complete` as repeatable string slice flags on the run command in `cmd/bolt/main.go`
- [ ] 4.2 Add `--hook-timeout` flag (time.Duration, default 30s)
- [ ] 4.3 Implement env-var resolution in a helper: read `BOLT_ON_FAILURE` etc, split on unescaped commas, respect flag-precedence rules
- [ ] 4.4 Implement backslash-comma unescaping for env var values
- [ ] 4.5 Parse and validate `BOLT_HOOK_TIMEOUT` as a duration; surface clear error on bad value

## 5. Runner

- [ ] 5.1 Implement `Runner.Run(ctx, payload *Payload)` — sequentially runs matched hooks based on payload.status, then on_complete
- [ ] 5.2 For each hook: spawn `/bin/sh -c <cmd>` with `exec.CommandContext`, pass payload JSON on stdin, close stdin after write
- [ ] 5.3 Set hook env: inherit process env + add `BOLT_RUN_ID`, `BOLT_RUN_STATUS`, `BOLT_PLAYBOOK`
- [ ] 5.4 Capture combined stdout+stderr to a 64KB-bounded buffer with truncation suffix
- [ ] 5.5 Enforce timeout via ctx with `WithTimeout`; on cancel, send SIGTERM, wait 2s, then SIGKILL
- [ ] 5.6 Emit warning to stderr on hook failure/timeout (includes cmd + reason); show output when verbose OR when hook failed
- [ ] 5.7 Never propagate hook errors to bolt's exit code — Runner.Run returns nil even on hook failures

## 6. UUID & Run Identity

- [ ] 6.1 Add small UUIDv4 generator (crypto/rand based; 16 lines) OR confirm an existing dependency provides one; avoid pulling a new lib just for this
- [ ] 6.2 Generate `run_id` once at start of run and thread it through to payload + env vars

## 7. Wiring

- [ ] 7.1 In `cmd/bolt/main.go` run command: after executor returns, build payload from summary, then call `hooks.Runner.Run`
- [ ] 7.2 Ensure summary output is flushed BEFORE hook runner starts
- [ ] 7.3 Propagate run_id into bolt log output so users can correlate runs with hook payloads

## 8. Tests

- [ ] 8.1 Unit test payload builder: success path, failed-host path, unreachable counts as failed, empty hosts list
- [ ] 8.2 Unit test CLI flag+env precedence (flag wins, env-only, comma splitting, backslash-comma escape)
- [ ] 8.3 Unit test timeout behavior with a sleep 5 command and a 100ms timeout
- [ ] 8.4 Unit test output truncation at 64KB boundary
- [ ] 8.5 Unit test env-var injection: hook script writes `$BOLT_RUN_ID` to a file; test verifies
- [ ] 8.6 Unit test hook failure does not change bolt exit code
- [ ] 8.7 Unit test redaction: no_log task message is redacted in payload
- [ ] 8.8 Integration test: tiny bash script captures stdin to a tmp file, bolt invokes it via `--on-complete`, test parses the JSON and verifies schema_version + fields
- [ ] 8.9 Integration test: multiple `--on-failure` flags run in registration order
- [ ] 8.10 `go test -race ./...` passes

## 9. Documentation

- [ ] 9.1 Add `docs/hooks.md` with flag reference, env var reference, full payload example, examples (Slack via curl, metrics emission, audit log append), control-host-only note
- [ ] 9.2 Update `README.md` with a "Notifications" section
- [ ] 9.3 Update `llms.txt` with hook flag reference
- [ ] 9.4 Add CI snippet example showing `--on-failure` with curl posting to Slack
- [ ] 9.5 Update `ROADMAP.md` — mark event hooks (CLI) as implemented

## 10. Release

- [ ] 10.1 Run `make lint` and `make test`
- [ ] 10.2 Manual smoke: invoke a failing playbook with `--on-failure "cat > /tmp/bolt-payload.json"` and verify payload
- [ ] 10.3 Manual smoke: verify timeout kills a `sleep 60` hook with `--hook-timeout 1s`
