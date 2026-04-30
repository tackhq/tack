# cron

Manage individual cron entries idempotently on Linux targets — create,
update, comment-out (disable), and remove scheduled jobs in user crontabs
or `/etc/cron.d/` drop-in files. Each managed entry is preceded by a
`# TACK: <name>` marker comment that the module uses to locate and
compare lines on subsequent runs.

Linux only. macOS, BSD, and Windows targets fail with a clear error.

## Parameters

| Param          | Type   | Required | Default | Description                                                                                                                       |
|----------------|--------|----------|---------|-----------------------------------------------------------------------------------------------------------------------------------|
| `name`         | string | yes      | -       | Identifier written into the marker comment (`# TACK: <name>`). Unique per crontab. ≤200 chars; no newlines, `#`, or non-printable.|
| `job`          | string | when present | -   | Command to run. Required when `state: present` and `env: false`. For `env: true`, must be `KEY=VALUE`.                            |
| `state`        | string | no       | present | `present` or `absent`.                                                                                                            |
| `minute`       | string | no       | `*`     | Minute schedule field.                                                                                                            |
| `hour`         | string | no       | `*`     | Hour schedule field.                                                                                                              |
| `day`          | string | no       | `*`     | Day-of-month schedule field.                                                                                                      |
| `month`        | string | no       | `*`     | Month schedule field.                                                                                                             |
| `weekday`      | string | no       | `*`     | Day-of-week schedule field.                                                                                                       |
| `special_time` | string | no       | -       | One of `reboot`, `yearly`, `annually`, `monthly`, `weekly`, `daily`, `hourly`. Mutually exclusive with the time fields.           |
| `user`         | string | no       | (current) | When `cron_file` is unset: whose crontab to edit (`crontab -u <user>`). When `cron_file` is set: the user field written into the line (default `root`). |
| `cron_file`    | string | no       | -       | Absolute path to a `/etc/cron.d/` drop-in file. Basename must match `^[A-Za-z0-9_-]+$` (no dots, no extensions).                  |
| `disabled`     | bool   | no       | false   | When true, the managed line is prefixed with `# ` (commented out) while the marker is preserved.                                  |
| `env`          | bool   | no       | false   | When true, `job` is treated as a `KEY=VALUE` environment line; schedule fields are rejected.                                      |

## Result fields (for `register:`)

| Field     | Description                                                                              |
|-----------|------------------------------------------------------------------------------------------|
| `changed` | True when the crontab/drop-in content was modified.                                      |
| `action`  | One of `created`, `updated`, `removed`, `disabled`, `enabled`, `unchanged`, `already-absent`. |
| `file`    | Backend label: drop-in path, `crontab for user <user>`, or `crontab`.                    |
| `name`    | The marker name.                                                                         |

## Prerequisites

- The `crontab` binary must be installed on the target. Most Debian/Ubuntu
  images need `apt-get install -y cron`; RHEL-family needs `cronie`.
- Writing to another user's crontab via `crontab -u <user>` typically
  requires root — enable `sudo: true` on the play or task.
- Writing to `/etc/cron.d/` requires root.

## Examples

### Daily backup in the current user's crontab

```yaml
- name: Schedule a daily backup
  cron:
    name: backup
    job: /usr/local/bin/backup.sh
    hour: "2"
    minute: "0"
```

After this runs the user's crontab contains:

```
# TACK: backup
0 2 * * * /usr/local/bin/backup.sh
```

Re-running the same task produces no changes.

### Set a PATH environment line

```yaml
- name: Pin PATH for cron jobs
  cron:
    name: path
    env: true
    job: "PATH=/usr/local/bin:/usr/bin:/bin"
```

Result:

```
# TACK: path
PATH=/usr/local/bin:/usr/bin:/bin
```

### /etc/cron.d drop-in with a special-time shortcut

```yaml
- name: Hourly health check
  cron:
    name: health-check
    cron_file: /etc/cron.d/health-check
    user: root
    job: /usr/local/bin/healthcheck.sh
    special_time: hourly
  sudo: true
```

Result in `/etc/cron.d/health-check`:

```
# TACK: health-check
@hourly root /usr/local/bin/healthcheck.sh
```

### Temporarily disable a job

```yaml
- name: Disable the report job during maintenance
  cron:
    name: report
    job: /usr/local/bin/report.sh
    special_time: daily
    disabled: true
```

The marker stays in place; the schedule line is prefixed with `# `. Set
`disabled: false` (or omit it) to re-enable.

### Remove an entry

```yaml
- name: Drop the obsolete cleanup job
  cron:
    name: old-cleanup
    state: absent
```

Both the marker and schedule line are removed. When the last entry in a
`cron_file` is removed, the file itself is deleted.

### Manage another user's crontab

```yaml
- name: Schedule a job in alice's crontab
  cron:
    name: alice-report
    user: alice
    job: /home/alice/bin/report.sh
    hour: "6"
    minute: "30"
  sudo: true
```

## Notes

- Managed lines are single-line. Hand-editing the schedule line preserves
  idempotency only if you keep the line single-line and unmodified above
  the marker.
- The module does not lock the crontab. Concurrent tack runs editing the
  same crontab can race; partition playbooks or use `--forks 1` if it
  matters.
- `--dry-run` and `--diff` are supported and show the prospective
  crontab/drop-in content without modifying the target.
