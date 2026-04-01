# Bolt Roadmap

Feature roadmap based on team discussion covering PM, DevOps (senior/mid/junior), developer, and IT automation perspectives.

## P0 — Structural (unlocks new playbook patterns)

### `include_tasks` / `import_tasks`
Include shared task files inline without wrapping them in a full role.
- `import_tasks` — static, resolved at parse time
- `include_tasks` — dynamic, resolved at runtime, respects `when:`
- **Why:** Eliminates YAML duplication across playbooks; unblocks complex compositions

### `block` / `rescue` / `always`
Group tasks with structured error handling.
- `block:` — list of tasks to attempt
- `rescue:` — runs if any block task fails
- `always:` — runs regardless of outcome
- **Why:** Required for any stateful workflow involving rollback (database migrations, blue-green deploys, etc.)

## P1 — High Impact

### Tags
Selective task execution via `--tags` and `--skip-tags` CLI flags.
- Tags on tasks and roles
- Tag inheritance through blocks/roles
- **Why:** Large playbooks become unusable without selective execution

### `user` + `group` modules
Idempotent user and group provisioning.
- `user`: name, state, groups, shell, home, uid, password (hashed), system, remove
- `group`: name, state, gid, system
- Optional `ssh_authorized_keys` management
- **Why:** Core provisioning primitive — currently requires brittle `command` tasks

### `lineinfile` / `blockinfile` modules
Surgical file edits without full template management.
- `lineinfile`: regexp, line, state, insertafter/insertbefore, backup
- `blockinfile`: marker, block, state, insertafter/insertbefore, backup
- **Why:** Fills the gap between whole-file `copy`/`template` and manual `command` edits

### Dynamic inventory
Support external inventory sources beyond static YAML.
- Script/binary inventory plugin: run command, parse JSON output
- Built-in AWS EC2 plugin (extend existing SSM tag discovery)
- Generic HTTP inventory source
- **Why:** Static YAML doesn't scale for cloud-native fleets where hosts come and go

## P2 — Quality of Life

### `bolt export` — compile playbook to standalone shell script
Export a playbook or role as a self-contained bash script for debugging, auditing, or offline use.
- Captures the shell commands Bolt would send through a connector
- Resolves variables, templates, and conditionals at export time
- Produces a portable script runnable without Bolt installed
- **Why:** Visibility into exactly what Bolt executes; useful for security audits, air-gapped hosts, and debugging module behavior

### `--diff` mode
Show file content diffs for `copy`, `template`, and `file` modules before applying.
- Works with `--dry-run` for full preview
- Colored unified diff output
- **Why:** Best feature for reviewing infra changes before they happen

### `wait_for` module
Poll for conditions before proceeding.
- Params: port, host, path, status_code, timeout, delay, state (started/stopped)
- **Why:** Replaces fragile `command` shell loops for service readiness checks

### `assert` module
Validate preconditions and fail fast with clear messages.
- Params: that (list of conditions), fail_msg, success_msg
- **Why:** Catch misconfigurations early instead of cryptic failures mid-playbook

### `cron` module
Manage scheduled jobs idempotently.
- Params: name, job, minute/hour/day/month/weekday, state, user
- Managed comment markers in crontab for idempotency
- **Why:** Log rotation, backups, cert renewal — common on every server

### `git` module
Manage git repository checkouts on targets.
- Params: repo, dest, version/ref, force, depth, accept_hostkey
- Idempotent: skip if already at desired ref
- **Why:** Core app deployment primitive

### Event hooks / callbacks
Pluggable notifications on run events.
- Hooks: on_task_start, on_task_fail, on_play_complete
- Webhook support for Slack, PagerDuty, etc.
- Quick-win: `--on-failure "cmd"` CLI flag
- **Why:** Observability and alerting for production runs

## P3 — Nice to Have

### `docker` module
Manage containers as desired state on targets (separate from the Docker connector).
- Params: image, name, state, ports, env, volumes, restart_policy
- Optional `docker_compose` variant with compose file hash idempotency
- **Why:** Container lifecycle management without external scripts

### `sysctl` module
Manage kernel parameters idempotently.
- Params: name, value, state, reload, sysctl_file
- Persist to `/etc/sysctl.d/` with reload
- **Why:** Server hardening and performance tuning

### `ufw` / `firewalld` modules
Manage firewall rules declaratively.
- `ufw`: rule, port, proto, direction, state, from_ip, to_ip
- `firewalld`: service, port, zone, state, permanent, immediate
- **Why:** Security hardening without error-prone shell commands

### Exponential backoff for retries
Extend existing `retries` + `delay` with `delay_factor` for exponential backoff.
- **Why:** Prevents thundering-herd issues when hitting rate-limited APIs at scale with `--forks`
