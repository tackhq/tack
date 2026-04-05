## Why

Tack currently has no way to manage system users and groups. These are fundamental primitives for server provisioning -- nearly every playbook that sets up a system needs to create service accounts, manage group memberships, or ensure specific users exist. Without user/group modules, users must fall back to raw `command` tasks, losing idempotency guarantees and cross-platform portability.

## What Changes

- Add a `user` module for creating, modifying, and removing system users with parameters for name, state, groups, shell, home directory, uid, hashed password, system user flag, and remove-on-absent behavior.
- Add a `group` module for creating, modifying, and removing system groups with parameters for name, state, gid, and system group flag.
- Both modules follow the existing Module interface pattern, register via `init()`, and use OS commands (`useradd`/`usermod`/`userdel`, `groupadd`/`groupmod`/`groupdel`) through the connector for remote execution.
- Both modules are fully idempotent -- they query current state before making changes and only act when the desired state differs from reality.

## Capabilities

### New Capabilities
- `user-module`: Idempotent user account provisioning (create, modify, remove) with support for groups, shell, home, uid, password, and system users.
- `group-module`: Idempotent group provisioning (create, modify, remove) with support for gid and system groups.

### Modified Capabilities

None.

## Impact

- New packages: `internal/module/user/`, `internal/module/group/`
- Module registry: two new modules auto-registered via `init()`
- No changes to existing code, APIs, or dependencies
- No new external dependencies required (uses standard OS commands via connector)
- Playbook shorthand expansion in `internal/playbook/` will need the module names added to recognition (if applicable)
