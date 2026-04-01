## Why

Bolt currently supports `apt` (Debian/Ubuntu) and `brew` (macOS) for package management but has no support for RPM-based distributions. RHEL, CentOS, Fedora, Amazon Linux, and Rocky Linux use `yum` or `dnf` as their package managers. Without this module, users cannot manage packages on a large class of production Linux systems.

## What Changes

- Add a new `yum` module that manages packages via `yum` or `dnf`, auto-detecting which is available on the target system
- Support installing, removing, upgrading, and querying packages with full idempotency
- Support cache updates, upgrade-all, and autoremove operations
- Implement dry-run support via the `Checker` interface
- Register the module in the CLI entrypoint

## Capabilities

### New Capabilities
- `yum-module`: Package management module for RPM-based Linux distributions (RHEL, CentOS, Fedora, Amazon Linux, Rocky Linux) supporting install, remove, upgrade, and cache operations via yum/dnf

### Modified Capabilities

_None — this is a new module with no changes to existing capabilities._

## Impact

- **New code**: `internal/module/yum/` package with module implementation and tests
- **Modified code**: `cmd/bolt/main.go` — add blank import for module registration
- **No dependency changes** — uses only the standard library and existing bolt interfaces
- **No breaking changes** — additive feature only
