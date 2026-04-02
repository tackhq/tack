## Context

Bolt modules follow a consistent pattern: a struct implementing `Module` (with `Name()` and `Run()`), auto-registered via `init()`, using the connector to execute OS commands on targets. Existing modules like `apt`, `brew`, and `systemd` demonstrate this pattern well.

User and group management on Linux uses the standard `useradd`/`usermod`/`userdel` and `groupadd`/`groupmod`/`groupdel` commands. Current state can be queried by parsing `/etc/passwd` and `/etc/group` via the connector. macOS uses `dscl` for user/group management, but this design targets Linux first (macOS support is a non-goal for this iteration).

## Goals / Non-Goals

**Goals:**
- Idempotent user creation, modification, and removal via `useradd`/`usermod`/`userdel`
- Idempotent group creation, modification, and removal via `groupadd`/`groupmod`/`groupdel`
- Support for common parameters: name, state, groups, shell, home, uid, gid, hashed password, system flag
- Check mode (dry-run) support via the `Checker` interface
- Follow existing module patterns exactly for consistency

**Non-Goals:**
- macOS (`dscl`) support -- can be added later
- FreeBSD/other OS support
- Managing SSH authorized keys (separate concern)
- Password hashing within the module -- passwords must be provided pre-hashed
- Managing user crontabs or resource limits

## Decisions

### 1. State detection via /etc/passwd and /etc/group parsing

Parse `/etc/passwd` (for users) and `/etc/group` (for groups) to determine current state before making changes. This avoids relying on command exit codes for existence checks, which vary across distributions.

**Alternative considered**: Using `getent passwd <name>` / `getent group <name>`. This would also work but parsing the output is equivalent to parsing `/etc/passwd` format. `getent` is slightly more correct when LDAP/NIS is in use, but Bolt targets local system accounts. We will use `getent` as it handles both local and networked sources.

### 2. Separate user and group modules

Two distinct modules (`user` and `group`) rather than a single combined module. This matches how Ansible structures them and allows playbooks to manage groups independently before assigning users to them.

**Alternative considered**: Single `account` module with a `type` parameter. Rejected because it conflates two distinct operations and makes playbooks less readable.

### 3. Supplementary groups via usermod -G

For user group membership, use `usermod -aG` to append supplementary groups. The `groups` parameter accepts a list. If `groups` is set, the module compares current supplementary groups and only calls `usermod` if they differ.

### 4. Password handling

The `password` parameter accepts a pre-hashed password string (e.g., SHA-512 crypt format). The module passes it to `useradd -p` or `usermod -p`. The module will NOT hash plaintext passwords -- this is a security decision to avoid storing plaintext in playbooks.

### 5. Remove flag for user deletion

When `state: absent`, the `remove` parameter (default: false) controls whether the user's home directory is also removed (`userdel -r`). This prevents accidental data loss.

## Risks / Trade-offs

- **[Risk] Command availability**: `useradd`/`groupadd` may not exist on minimal container images. → Mitigation: Module returns a clear error if commands are not found, similar to how `apt` module checks for apt availability.
- **[Risk] Password in process listing**: `useradd -p <hash>` exposes the hash in `ps` output briefly. → Mitigation: This is the standard approach (same as Ansible). The hash is not the plaintext password. For higher security, users can use `chpasswd` via a command task.
- **[Risk] Concurrent modification**: If multiple Bolt runs modify users simultaneously, race conditions could occur. → Mitigation: Out of scope; Bolt does not provide distributed locking. Document as known limitation.
- **[Trade-off] Linux-only**: No macOS support initially. This is acceptable because server provisioning (Bolt's primary use case) targets Linux almost exclusively.
