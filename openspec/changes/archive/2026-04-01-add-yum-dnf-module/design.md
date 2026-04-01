## Context

Bolt has a well-established module pattern for package management with `apt` (Debian/Ubuntu) and `brew` (macOS). Both follow the same `Module` interface: self-registration via `init()`, parameter extraction via helpers, idempotency through state queries before action, and dry-run support via the `Checker` interface. RPM-based distributions (RHEL, CentOS, Fedora, Amazon Linux, Rocky Linux) represent a large share of production Linux servers but are currently unsupported.

The two RPM package managers — `yum` and `dnf` — share nearly identical CLI interfaces. `dnf` is the successor to `yum` and is the default on Fedora 22+ and RHEL 8+. On many systems, `yum` is a symlink to `dnf`. Both use `rpm` as the underlying package database.

## Goals / Non-Goals

**Goals:**
- Provide a `yum` module that manages packages on RPM-based systems
- Auto-detect whether to use `dnf` or `yum` on the target system
- Support the same parameter patterns as `apt` where applicable (name, state, update_cache, upgrade, autoremove)
- Full idempotency: query package state via `rpm` before taking action
- Dry-run support via the `Checker` interface

**Non-Goals:**
- Repository management (adding/removing yum repos) — future module
- Managing specific package versions (e.g., `nginx-1.24.0`) — future enhancement
- `zypper` (SUSE) support — separate module
- DNF5 module system (modularity streams) — niche feature, out of scope

## Decisions

### 1. Single module named `yum` with auto-detection

The module will be named `yum` (not `dnf` or `rpm`) since `yum` is the more universally recognized name and works on both old and new systems. At runtime, the module will check for `dnf` first, falling back to `yum`. This mirrors Ansible's approach.

**Alternative considered:** Separate `yum` and `dnf` modules. Rejected because the CLI interfaces are nearly identical, and users shouldn't need to know which one their target uses.

### 2. Use `rpm -q` for state queries

Package state will be queried using `rpm -q <package>` rather than `yum list installed`. `rpm` is faster (no metadata fetch), always available, and gives deterministic output. This is the same pattern used by the `apt` module with `dpkg-query`.

### 3. Follow the apt module structure closely

The implementation will mirror `apt.go` in structure: same parameter names where applicable (`name`, `state`, `update_cache`), same result patterns (`Changed`/`Unchanged`), same `Check()` method approach. This keeps the codebase consistent and the user-facing API predictable.

### 4. State values: present, absent, latest

Matching the `apt` and `brew` modules. No `purged` state since RPM doesn't distinguish between remove and purge the way dpkg does.

## Risks / Trade-offs

- **[Risk] yum/dnf output parsing** — Output formats can vary across versions. → Mitigation: Use `rpm -q` for state queries (stable format) and rely on exit codes rather than parsing output for install/remove operations.
- **[Risk] Privilege escalation** — Package management requires root. → Mitigation: Bolt's connector layer already handles sudo transparently; no special handling needed in the module.
- **[Trade-off] No version pinning** — Users cannot install a specific version. → Acceptable for initial implementation; can be added later without breaking changes.
