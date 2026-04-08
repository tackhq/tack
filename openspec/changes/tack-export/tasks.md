## 1. Package Scaffold

- [x] 1.1 Create `internal/export/` package with `export.go` (orchestrator), `script.go` (banner/trap/footer), `block.go` (per-task block rendering), `loop.go` (loop unroll), `include.go` (include inlining)
- [x] 1.2 Define `EmitResult` struct (Supported, Reason, Shell, PreHook, Warnings) in `internal/module/module.go`
- [x] 1.3 Define `Emitter` interface in `internal/module/module.go` alongside `Checker`
- [x] 1.4 Define `Compiler` struct holding playbook, host, vars, facts, options

## 2. CLI Subcommand

- [x] 2.1 Add `export` command to `cmd/tack/main.go` via cobra
- [x] 2.2 Wire flags: `--host`, `--all-hosts`, `--output`, `--no-facts`, `--check-only`, `-e`/`--extra-vars`, `--tags`, `--skip-tags`, `--connection`, `--no-banner-timestamp`
- [x] 2.3 Validation: `--host` XOR `--all-hosts`; `--all-hosts` requires `--output` as directory; reject simultaneous flags
- [x] 2.4 Default host selection: single inventory match when neither flag set
- [x] 2.5 Parse `--extra-vars` with same helper used by `run` command

## 3. Fact Freezing

- [x] 3.1 Implement `gatherFactsForExport(ctx, conn, host) map[string]any` reusing `pkg/facts`
- [x] 3.2 Freeze facts once per host into in-memory map
- [x] 3.3 `--no-facts` path: substitute sentinel `__TACK_FACT_NOT_GATHERED__` and set warning flag
- [x] 3.4 Render facts alphabetically into banner comment, skipping values that exceed 200 chars

## 4. Variable Resolution Context

- [x] 4.1 Build export-time `PlayContext` equivalent: extra vars ∪ play vars ∪ host vars ∪ frozen facts (match runtime precedence)
- [x] 4.2 Resolve `{{ var }}` interpolations during compilation using existing interpolation helper
- [x] 4.3 Detect references to registered variables → mark task as "runtime dependency" for later handling

## 5. `when:` Evaluation

- [x] 5.1 Evaluate each task's `when:` at export time using `evaluateConditionExpr`
- [x] 5.2 False → emit `# SKIPPED (when false): <expr>` and skip task emission
- [x] 5.3 Expression references a registered variable → emit task with `# WARN: when references runtime variable` comment, include unconditionally
- [x] 5.4 Malformed expression → export fails with clear error naming the task

## 6. Tag Filtering

- [x] 6.1 Apply `--tags`/`--skip-tags` filter BEFORE emission using existing tag-selector logic
- [x] 6.2 Pass effective-tag list per surviving task into block renderer
- [x] 6.3 Sort tags alphabetically for deterministic header output

## 7. Loop Expansion

- [x] 7.1 Detect `loop:` with static list → unroll at export time, binding `item` per iteration
- [x] 7.2 Detect `loop:` with resolvable variable → resolve and unroll
- [x] 7.3 Detect `loop:` with runtime dependency → emit UNSUPPORTED
- [x] 7.4 Preserve input list ordering

## 8. Static include_tasks

- [x] 8.1 Detect static include (no loop, no interpolated path)
- [x] 8.2 Recursively compile included tasks in place
- [x] 8.3 Reuse existing circular-include detection (max depth 64)
- [x] 8.4 Dynamic include → emit UNSUPPORTED

## 9. Script Skeleton Rendering

- [x] 9.1 Implement `renderBanner(ctx Compiler)` producing shebang + set -euo pipefail + banner comments
- [x] 9.2 Include tack version, playbook path, host, timestamp (UTC, second precision), facts summary
- [x] 9.3 `--no-banner-timestamp` drops the timestamp line
- [x] 9.4 Render counter declarations + trap function + trap install
- [x] 9.5 Add vault-values-present warning block when any decrypted vault var was resolved
- [x] 9.6 Render summary footer (empty; trap handles it)

## 10. Per-Task Block Rendering

- [x] 10.1 Implement `renderBlock(task, result EmitResult)` producing `# === TASK: ... ===` header + `TACK_CURRENT_TASK=` + PreHook + Shell + changed-counter bump guard
- [x] 10.2 Deduplicate PreHook fragments across blocks
- [x] 10.3 Wrap shell in `>/dev/null 2>&1` redirect when `no_log: true`
- [x] 10.4 Render UNSUPPORTED block for tasks without Emit: `# UNSUPPORTED: <reason>` + embedded task YAML (redacted) as comments
- [x] 10.5 Emit block/rescue/always + handlers as UNSUPPORTED (no partial emission in v1)

## 11. Module Emitter Implementations

- [ ] 11.1 Refactor each supported module's Run path to call a pure `buildCommand(...)` function — shared between Run and Emit
- [x] 11.2 Implement `Emit` on `command` module (with changed_when + creates/removes guards)
- [x] 11.3 Implement `Emit` on `apt` module (apt-get install/remove with dpkg-check-before)
- [x] 11.4 Implement `Emit` on `brew` module
- [x] 11.5 Implement `Emit` on `yum` module
- [x] 11.6 Implement `Emit` on `file` module (state: touch/directory/absent/link)
- [x] 11.7 Implement `Emit` on `copy` module (heredoc or base64 + diff-guard + chmod/chown)
- [x] 11.8 Implement `Emit` on `template` module (render at export + heredoc + diff-guard + chmod/chown)
- [x] 11.9 Implement `Emit` on `lineinfile` module (sed-based guarded edits)
- [x] 11.10 Implement `Emit` on `blockinfile` module (awk/sed block management)
- [x] 11.11 Implement `Emit` on `systemd` module (systemctl invocations with state/enabled check)
- [x] 11.12 Implement `Emit` on `user` module (id + useradd/usermod/userdel pattern)
- [x] 11.13 Implement `Emit` on `group` module (getent + groupadd/groupmod/groupdel)
- [ ] 11.14 (When merged) Implement `Emit` on `assert` — emit bash guard that evaluates the conditions as shell tests
- [x] 11.15 (When merged) Implement `Emit` on `cron` module (crontab editor shell logic)
- [x] 11.16 (When merged) Implement `Emit` on `git` module (clone/fetch/checkout with SHA guard)

## 12. Output Writing

- [x] 12.1 Single-host mode: write to `--output` path or stdout
- [x] 12.2 All-hosts mode: for each host, sanitize hostname, write `<dir>/<host>.sh`
- [x] 12.3 All-hosts mode: emit `INDEX.txt` listing produced files
- [x] 12.4 Create output directory if missing (all-hosts)
- [x] 12.5 Set output file mode to 0600 (vault concern)

## 13. `--check-only` Mode

- [x] 13.1 Run compilation but skip file writes
- [x] 13.2 Collect list of supported tasks and unsupported constructs with reasons
- [x] 13.3 Print summary table (task count, supported/unsupported, per-unsupported reason)
- [x] 13.4 Exit non-zero if any unsupported construct found

## 14. Determinism

- [x] 14.1 Sort all map iteration (task params when emitting, env vars, facts, tags)
- [ ] 14.2 Unit test: same input → byte-identical output (with --no-banner-timestamp)
- [ ] 14.3 Document determinism guarantees

## 15. Tests

- [x] 15.1 Unit tests for each module's Emit (golden files per module + representative param sets)
- [ ] 15.2 Golden-file tests: full playbook → full script for 3-5 representative playbooks
- [x] 15.3 Unit tests for loop expansion (static list, variable list, runtime list)
- [x] 15.4 Unit tests for when: pruning (true, false, runtime-var)
- [x] 15.5 Unit tests for tag filtering applied during export
- [x] 15.6 Unit tests for UNSUPPORTED emission (async, handlers, block/rescue/always, registry-miss)
- [ ] 15.7 Unit tests for no_log wrapping and embedded-YAML redaction
- [ ] 15.8 Unit tests for vault warning banner presence/absence
- [x] 15.9 Unit tests for `--no-facts` sentinel substitution + banner warning
- [x] 15.10 Unit tests for deterministic output (diff between two export runs = empty)
- [x] 15.11 Unit tests for `--check-only` exit codes
- [x] 15.12 Integration test: export a playbook, run the emitted script inside a Docker container, verify end state matches running tack normally
- [x] 15.13 Integration test: re-run the emitted script twice and verify second run produces `changed=0` (idempotency)
- [x] 15.14 Integration test: `--all-hosts` produces N files with correct per-host variable substitution
- [x] 15.15 `go test -race ./...` passes

## 16. Documentation

- [ ] 16.1 Add `docs/export.md` with overview, flag reference, supported/unsupported construct matrix, examples, security notes (vault, secrets in plaintext), air-gapped workflow, audit workflow
- [ ] 16.2 Update `README.md` with an "Export" section
- [ ] 16.3 Update `llms.txt` with `tack export` usage
- [ ] 16.4 Add `examples/export-audit/` showing a playbook + exported script + diff-review workflow
- [ ] 16.5 Update `ROADMAP.md` — mark `tack export` as implemented, celebrating P2 completion

## 17. Release

- [ ] 17.1 Run `make lint` and `make test`
- [ ] 17.2 Manual smoke: export the vault example playbook, verify banner contains vault warning, inspect script for secrets
- [ ] 17.3 Manual smoke: `--all-hosts` against a 3-host inventory; verify per-host variable substitution
- [ ] 17.4 Manual smoke: run the emitted script in a clean Docker container and verify success
- [ ] 17.5 Manual smoke: `--check-only` on a playbook with async task returns non-zero with clear report
