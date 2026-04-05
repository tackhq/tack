## Context

Cron has two management surfaces on Linux:
1. **User crontabs** managed by the `crontab` binary (`crontab -l` to read, `crontab -` to replace, `-u <user>` to target another user). Storage location varies (`/var/spool/cron/`, `/var/spool/cron/crontabs/`) and is not meant to be edited directly.
2. **System drop-ins** in `/etc/cron.d/` — plain files read by the cron daemon. Each line includes a `user` field between schedule and command. Drop-ins must pass the cron daemon's filename rules (no dots, limited charset).

Tack's existing modules all execute via the `Connector` interface, so the cron module composes `crontab` invocations and file reads/writes rather than touching spool files directly. Similar precedent exists in `internal/module/user/` and `internal/module/systemd/` which both call OS binaries idempotently.

The connector already supports `Execute`, `Upload`, `Download`, and sudo. No interface changes required.

## Goals / Non-Goals

**Goals:**
- Idempotent create/update/remove/disable of one cron entry per task invocation, identified by a managed comment marker.
- Support user crontabs and `/etc/cron.d/` drop-ins through the same module.
- Faithful `--dry-run` + `--diff` that shows the prospective crontab content.
- Clean error path on non-Linux targets.

**Non-Goals:**
- Systemd timers — separate module scope.
- `/etc/crontab` edits — discouraged; users should use `/etc/cron.d/` drop-ins.
- Environment-line ordering beyond "keep env lines together at the top" — out of scope.
- macOS launchd bridging — separate module scope.
- Bulk import/export of entire crontabs — one entry per task, Ansible-style.

## Decisions

### Decision 1: Marker-based idempotency, not line-content hashing

Each managed entry is preceded by a single-line marker comment: `# BOLT: <name>`. The module locates managed lines by scanning for the marker, then compares the following line (schedule + command) to the desired state. A mismatch triggers rewrite.

**Alternatives considered:**
- **Hash suffix in comment** (`# BOLT: name [hash]`): Cleaner conflict detection but surprises users who hand-edit the line. Rejected — humans need to be able to read and edit the managed line without breaking idempotency.
- **External state file on target**: Too much state. Rejected.

**Rationale:** Matches Ansible's long-standing pattern; users already recognize it. The marker is the source of truth for "Tack manages this line."

### Decision 2: Read-modify-write for user crontabs

For user crontabs the flow is: `crontab -l [-u user]` → in-memory edit → `crontab - [-u user]` to replace. When `crontab -l` exits non-zero with "no crontab for <user>" the module treats that as an empty crontab (not an error). When adding to an empty crontab, the module creates it.

For `/etc/cron.d/` drop-ins the flow is: `Download` file (or empty string if missing) → in-memory edit → `Upload` with 0644 mode. When removing the last entry from a drop-in, the module deletes the file rather than leaving it empty.

### Decision 3: `special_time` and time fields are mutually exclusive

If both are specified, the task fails validation. When `special_time` is set, the schedule line begins with `@reboot` / `@daily` / etc. When time fields are set, defaults are `"*"` for each unspecified one.

### Decision 4: `user` vs `cron_file` are mutually exclusive

Specifying both is a validation error. If neither is specified, the module uses the user crontab of whatever user the connector is authenticated as (which may be root when sudo is active — document this clearly).

For `/etc/cron.d/` drop-ins, the cron line includes the user field: `<schedule> <user> <command>`. The `user` param is required when `cron_file` is set (default `root`), and a separate param `file_user` is NOT introduced — we just overload `user` to mean "user to run the job" in both surfaces.

Wait — this creates ambiguity. **Resolution:** When `cron_file` is set, `user` names the user the cron line should run as (written into the line). When `cron_file` is unset, `user` names whose crontab to edit via `crontab -u`. Default in both cases: `root`.

### Decision 5: `disabled: true` comments the line, preserves marker

Disabling prepends `#` to the schedule+command line while leaving the `# BOLT: <name>` marker in place. Re-enabling removes the leading `#`. This preserves idempotency and avoids destroying the entry.

### Decision 6: `env: true` manages environment lines

When `env: true`, `job` must match `KEY=VALUE`. No schedule fields are required; validation rejects them. The module writes:
```
# BOLT: <name>
KEY=VALUE
```
This supports setting `PATH=...`, `MAILTO=...`, etc. idempotently.

### Decision 7: Linux-only, detect via facts

At task start, the module inspects `facts.os_type` (already gathered by `pkg/facts`). If not `Linux`, return an error with message: `cron module is only supported on Linux targets (got <os_type>); consider launchd on macOS or systemd-timers`. No silent no-op.

### Decision 8: Diff output shows full file context

For `--diff` mode, emit a unified diff between the current crontab (or drop-in file) and the prospective version, using Tack's existing diff helper. Small crontabs → show entire diff. Large crontabs → let the existing diff helper decide truncation.

### Decision 9: Name validation

The `name` param:
- Must be non-empty
- Must not contain newlines, `#`, or non-printable characters
- Length cap: 200 chars
Rejects with validation error on violation. This keeps marker comments parseable on re-read.

### Decision 10: Cron file name validation

When `cron_file` is set, the filename component (basename) must match cron daemon rules: only `[A-Za-z0-9_-]`, no dots, no extensions. Validation rejects with a message naming the forbidden characters. Path must be absolute and under `/etc/cron.d/` (warn-don't-fail if outside, to support containers).

### Decision 11: Concurrency

The module does NOT attempt to lock the crontab. Two concurrent tack runs editing the same user crontab can race. This matches Ansible's behavior and is documented. Users should partition playbooks or use `--forks 1` if this matters.

## Risks / Trade-offs

- **[Risk]** Users hand-edit the managed line and break format (e.g., wrap it to a second line). → **Mitigation:** On next run, the module detects the mismatch and rewrites. Document the expectation that managed lines are single-line.
- **[Risk]** The `crontab` binary's exit code / stderr format varies across distros (Vixie cron, cronie, systemd-cron packages). → **Mitigation:** Treat "no crontab" stderr patterns (English only: `no crontab for`) as empty; if unrecognized non-zero exit, surface stderr to user.
- **[Risk]** Under sudo, `crontab -u <user>` requires `root` or specific sudoers rules. Failure modes surface as cryptic errors. → **Mitigation:** When `sudo.enabled` is false and `user` != connected user, emit a preflight warning.
- **[Trade-off]** No bulk operations means managing many entries is verbose. → **Mitigation:** Users can use `loop:` (already supported) to iterate.
- **[Trade-off]** Markers leave a visual footprint in crontabs. → **Mitigation:** This is the accepted idiom; document it.
- **[Risk]** Drop-in file rewrites are not atomic through the connector abstraction if Upload doesn't use rename. → **Mitigation:** Check existing Upload semantics; if non-atomic, write to `<path>.tack.tmp` first and `mv` via Execute.
