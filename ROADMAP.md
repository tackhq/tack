# Bolt Roadmap

Feature roadmap based on team discussion covering PM, DevOps (senior/mid/junior), developer, and IT automation perspectives.

## Status Legend

- [ ] Not started
- [x] Implemented

## P0 — Structural (unlocks new playbook patterns)

| Status | Feature | Description | Details |
|--------|---------|-------------|---------|
| [x] | `include_tasks` | Enhanced task inclusion with `vars:`, `loop:`, circular detection | `include_tasks:` — dynamic runtime inclusion with scoped `vars:`, `loop:` support, variable-interpolated paths, circular detection (max depth 64). `include:` remains as alias. `import_tasks` deferred (unnecessary complexity). |
| [x] | `block` / `rescue` / `always` | Group tasks with structured error handling | `block:` tasks to attempt, `rescue:` on failure, `always:` runs regardless. Required for rollback workflows (DB migrations, blue-green deploys) |

## P1 — High Impact

| Status | Feature | Description | Details |
|--------|---------|-------------|---------|
| [x] | Tags | Selective task execution via `--tags` / `--skip-tags` | Tags on tasks, blocks, plays, and role references with inheritance. Special `always`/`never` tags. Handlers ignore `--tags` but respect `--skip-tags` |
| [ ] | `user` + `group` modules | Idempotent user and group provisioning | `user`: name, state, groups, shell, home, uid, password (hashed), system, remove. `group`: name, state, gid, system. Optional `ssh_authorized_keys` management |
| [x] | `lineinfile` / `blockinfile` | Surgical file edits without full template management | `lineinfile`: regexp, line, state, insertafter/insertbefore, backup. `blockinfile`: marker, block, state, insertafter/insertbefore, backup |
| [ ] | Dynamic inventory | External inventory sources beyond static YAML | Script/binary plugin (run command, parse JSON), built-in AWS EC2 plugin, generic HTTP source. Static YAML doesn't scale for cloud fleets |

## P2 — Quality of Life

| Status | Feature | Description | Details |
|--------|---------|-------------|---------|
| [ ] | `bolt export` | Compile playbook to standalone shell script | Captures shell commands Bolt would send through a connector. Resolves variables, templates, conditionals. Useful for security audits, air-gapped hosts, debugging |
| [x] | `--diff` mode | Show file content diffs before applying | Works with `--dry-run` for `copy`, `template`, `file` modules. Colored unified diff output |
| [x] | `wait_for` module | Poll for conditions before proceeding | Params: type (port/path/command/url), host, port, path, cmd, url, timeout, interval, state (started/stopped). Replaces fragile shell loops |
| [ ] | `assert` module | Validate preconditions and fail fast | Params: that (list of conditions), fail_msg, success_msg. Catch misconfigurations early |
| [ ] | `cron` module | Manage scheduled jobs idempotently | Params: name, job, minute/hour/day/month/weekday, state, user. Managed comment markers in crontab |
| [ ] | `git` module | Manage git repository checkouts on targets | Params: repo, dest, version/ref, force, depth, accept_hostkey. Idempotent: skip if already at desired ref |
| [ ] | Event hooks / callbacks | Pluggable notifications on run events | Hooks: on_task_start, on_task_fail, on_play_complete. Webhook support. Quick-win: `--on-failure "cmd"` CLI flag |

## P3 — Nice to Have

| Status | Feature | Description | Details |
|--------|---------|-------------|---------|
| [ ] | `docker` module | Manage containers as desired state on targets | Params: image, name, state, ports, env, volumes, restart_policy. Optional `docker_compose` variant with compose file hash idempotency |
| [ ] | `sysctl` module | Manage kernel parameters idempotently | Params: name, value, state, reload, sysctl_file. Persist to `/etc/sysctl.d/` |
| [ ] | `ufw` / `firewalld` modules | Manage firewall rules declaratively | `ufw`: rule, port, proto, direction, state. `firewalld`: service, port, zone, state, permanent, immediate |
| [ ] | Exponential backoff | Extend retries with `delay_factor` for backoff | Prevents thundering-herd issues when hitting rate-limited APIs at scale with `--forks` |
