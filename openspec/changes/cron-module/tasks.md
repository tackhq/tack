## 1. Package Scaffold

- [ ] 1.1 Create `internal/module/cron/` package with `cron.go` module struct, `Name() string`, and `init()` registration
- [ ] 1.2 Add `Describer` implementation with `Description()` and `Parameters()` (ParamDoc entries for all params)
- [ ] 1.3 Add `Checker` interface implementation stub for dry-run support

## 2. Parameter Parsing & Validation

- [ ] 2.1 Parse all params via `module/params.go` helpers (strings, bools, int ranges)
- [ ] 2.2 Validate mutually exclusive pairs: (`special_time` vs time fields), (`user` vs `cron_file`), (`env` vs time/special_time)
- [ ] 2.3 Validate `name`: non-empty, no newlines, no `#`, no non-printable, ≤200 chars
- [ ] 2.4 Validate `cron_file` basename against `^[A-Za-z0-9_-]+$`; require absolute path
- [ ] 2.5 Validate `special_time` is one of the allowed shortcuts
- [ ] 2.6 Validate `env: true` requires `job` matching `^[A-Za-z_][A-Za-z0-9_]*=.*$` and rejects time fields
- [ ] 2.7 Default handling: `state=present`, `disabled=false`, `env=false`, all time fields default to `*`, `user=root` when `cron_file` set

## 3. OS Gating

- [ ] 3.1 Read `facts.os_type` from PlayContext at entry; fail with explicit message if not Linux
- [ ] 3.2 Add test for macOS path returning a validation-style error before any connector call

## 4. User Crontab Backend

- [ ] 4.1 Implement `readUserCrontab(ctx, conn, user)` — invokes `crontab -l [-u user]`, treats "no crontab for" stderr as empty, surfaces unexpected errors
- [ ] 4.2 Implement `writeUserCrontab(ctx, conn, user, content)` — pipes content into `crontab - [-u user]` via connector Execute (use heredoc or stdin)
- [ ] 4.3 Apply sudo configuration when writing to another user

## 5. /etc/cron.d Drop-in Backend

- [ ] 5.1 Implement `readDropIn(ctx, conn, path)` using connector `Download` (treat missing file as empty)
- [ ] 5.2 Implement `writeDropIn(ctx, conn, path, content)` — write to `<path>.bolt.tmp` then `mv` for atomicity, mode 0644
- [ ] 5.3 Delete file when content is empty after edits

## 6. Crontab Editor (Core Logic)

- [ ] 6.1 Implement `locateManaged(lines, name)` — scan for `# BOLT: <name>` marker, return indices of marker + following line
- [ ] 6.2 Implement `renderScheduleLine(params)` — emit `@shortcut ` or `min hour day mon weekday `, then user field (drop-in only), then job
- [ ] 6.3 Implement `renderManagedBlock(name, scheduleLine, disabled)` — produces marker + (optionally-commented) line
- [ ] 6.4 Implement `applyPresent(lines, name, block)` — replace existing block in place, or append to end (with separating newline)
- [ ] 6.5 Implement `applyAbsent(lines, name)` — remove marker + following line
- [ ] 6.6 Implement `compareBlocks(existing, desired)` — byte-equal comparison for idempotency decision
- [ ] 6.7 Implement `applyDisabled(lines, name, disabled)` — toggle `# ` prefix on managed line, preserve marker

## 7. Module.Run Integration

- [ ] 7.1 Wire validation → OS check → backend read → edit → (dry-run?) → backend write
- [ ] 7.2 Return `Changed(msg)` / `Unchanged(msg)` based on before/after comparison
- [ ] 7.3 Populate `Result.Data` with `{action: "created|updated|removed|disabled|enabled|unchanged", file: "..."}` for register

## 8. Check Mode

- [ ] 8.1 Implement `Check(ctx, conn, params)` — perform read + edit, return `WouldChange` / `NoChange` without calling write backend
- [ ] 8.2 Ensure Check never modifies target

## 9. Diff Mode

- [ ] 9.1 On --diff, compute unified diff of old vs new crontab/drop-in content using existing diff helper
- [ ] 9.2 Attach diff to Result.Data so output layer can render it

## 10. Tests

- [ ] 10.1 Unit tests for crontab editor: locate/apply-present/apply-absent/disabled toggle/empty file/append/replace
- [ ] 10.2 Unit tests for parameter validation (every mutually-exclusive rule + name + file name + special_time + env)
- [ ] 10.3 Unit tests for render functions (time fields, special_time, env mode, drop-in with user field)
- [ ] 10.4 Module tests with mock connector covering: create, update, remove, disable/enable, already-matching (idempotency), empty-crontab handling, drop-in create/delete
- [ ] 10.5 Integration test against Docker container (debian with cron installed) — `tests/integration/cron_playbook.yaml` + Go test
- [ ] 10.6 Non-Linux smoke test — assert module returns error against macOS os_type fact
- [ ] 10.7 `go test -race ./...` passes

## 11. Documentation

- [ ] 11.1 Add `docs/modules/cron.md` with params table, examples (backup, PATH env, /etc/cron.d drop-in, disabled entry)
- [ ] 11.2 Update `README.md` feature list and module table
- [ ] 11.3 Update `llms.txt` with cron syntax
- [ ] 11.4 Add example playbook `examples/cron-backup.yaml`
- [ ] 11.5 Update `ROADMAP.md` — mark `cron` module as implemented

## 12. Release

- [ ] 12.1 Run `make lint` and `make test`
- [ ] 12.2 Validate example playbooks with `bolt validate`
- [ ] 12.3 Manual smoke test against local Docker cron container
