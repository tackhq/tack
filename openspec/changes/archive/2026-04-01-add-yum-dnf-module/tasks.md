## 1. Module Structure

- [x] 1.1 Create `internal/module/yum/` directory and `yum.go` file with `Module` struct and `init()` registration
- [x] 1.2 Implement `Name()` method returning `"yum"`

## 2. Package Manager Detection

- [x] 2.1 Implement helper to detect `dnf` vs `yum` on the target system (prefer `dnf`, fall back to `yum`, error if neither)

## 3. State Queries

- [x] 3.1 Implement `isInstalled()` using `rpm -q <package>` to check if a package is installed
- [x] 3.2 Implement `getUpdatable()` to check which packages have updates available (for `state: latest`)

## 4. Core Operations

- [x] 4.1 Implement package install (`state: present`) — skip already-installed packages, report changed only if at least one installed
- [x] 4.2 Implement package removal (`state: absent`) — skip not-installed packages, report changed only if at least one removed
- [x] 4.3 Implement package upgrade (`state: latest`) — install missing, upgrade outdated, report changed appropriately
- [x] 4.4 Implement cache update (`update_cache: true`) — run before package operations if both specified
- [x] 4.5 Implement upgrade-all (`upgrade: yes`) — upgrade all installed packages
- [x] 4.6 Implement autoremove (`autoremove: true`) — remove unused dependencies

## 5. Run Method

- [x] 5.1 Wire up the `Run()` method: parse parameters, detect package manager, execute operations in correct order (cache → install/remove/upgrade → autoremove)

## 6. Dry-Run Support

- [x] 6.1 Implement `Check()` method (Checker interface) — query state without modifications, return `WouldChange`/`NoChange`/`UncertainChange` results

## 7. Registration and Tests

- [x] 7.1 Add blank import `_ "github.com/eugenetaranov/bolt/internal/module/yum"` in `cmd/bolt/main.go`
- [x] 7.2 Write unit tests for the yum module covering install, remove, upgrade, cache, autoremove, and detection logic
