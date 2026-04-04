## Why

Scheduled jobs (backups, log rotation, health checks, periodic scripts) are a staple of system configuration, but Bolt has no idempotent way to manage them. Users must fall back to `command:` tasks that shell out to `crontab` or write crontab files by hand — both approaches lose idempotency guarantees and make playbooks brittle. A first-class `cron` module is the established Ansible pattern and unblocks common IT-automation workflows.

## What Changes

- Add a `cron` module that manages individual cron entries idempotently on Linux targets.
- Params: `name` (required, used as managed-comment marker), `job` (required when `state=present` and `env=false`), `minute`/`hour`/`day`/`month`/`weekday` (default `"*"`), `special_time` (shorthand: `reboot`/`yearly`/`annually`/`monthly`/`weekly`/`daily`/`hourly`; mutually exclusive with time fields), `user` (defaults to the connector's current user), `cron_file` (optional `/etc/cron.d/` drop-in path; mutually exclusive with `user`), `state` (`present`/`absent`, default `present`), `disabled` (bool — comment out the line without removing), `env` (bool — treat `job` as a `KEY=VALUE` environment line with no schedule).
- Idempotency uses a managed comment marker `# BOLT: <name>` placed immediately above each managed line. The module finds, compares, updates, or removes lines by matching the marker.
- Supports both user crontabs (`crontab -l` / `crontab -` [`-u <user>`]) and system cron drop-in files in `/etc/cron.d/`.
- Supports check mode (`--dry-run`) and `--diff` (emits a unified diff of the resulting crontab/drop-in contents).
- Returns a clear error on macOS/BSD/Windows targets — Linux only in this iteration.
- Works through every connector (local, SSH, SSM, Docker) by composing `crontab` invocations and file operations.

## Capabilities

### New Capabilities
- `cron-module`: Idempotent management of individual cron entries — create, update, comment-out (disable), and remove — in user crontabs and `/etc/cron.d/` drop-ins, with managed-comment markers for identification.

### Modified Capabilities

None.

## Impact

- New package: `internal/module/cron/`
- Module registry: one new module auto-registered via `init()`
- No changes to existing code, APIs, or dependencies
- No new external dependencies (uses `crontab` binary on target + connector file operations)
- Documentation: add `cron` to `README.md`, `docs/modules/`, `llms.txt`, and add an example playbook
- Shorthand expansion in `internal/playbook/` will need `cron` recognized (if that registry is explicit)
