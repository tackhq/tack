## 1. Group Module

- [x] 1.1 Create `internal/module/group/group.go` with `Module` struct, `Name()` returning `"group"`, and `init()` registration
- [x] 1.2 Implement `getGroupInfo()` helper that parses `getent group <name>` output to detect current group state (exists, gid)
- [x] 1.3 Implement `Run()` for group creation (`groupadd` with `-g` and `-r` flags based on params)
- [x] 1.4 Implement `Run()` for group modification (`groupmod -g` when gid differs)
- [x] 1.5 Implement `Run()` for group removal (`groupdel`)
- [x] 1.6 Implement `Check()` for dry-run support (Checker interface)
- [x] 1.7 Implement `Description()` and `Parameters()` for the Describer interface
- [x] 1.8 Add unit tests for group module in `internal/module/group/group_test.go`

## 2. User Module

- [x] 2.1 Create `internal/module/user/user.go` with `Module` struct, `Name()` returning `"user"`, and `init()` registration
- [x] 2.2 Implement `getUserInfo()` helper that parses `getent passwd <name>` output to detect current user state (exists, uid, shell, home, groups)
- [x] 2.3 Implement `getUserGroups()` helper that parses `id -Gn <name>` output to get supplementary group membership
- [x] 2.4 Implement `Run()` for user creation (`useradd` with `-s`, `-d`, `-u`, `-G`, `-r`, `-p` flags based on params)
- [x] 2.5 Implement `Run()` for user modification (`usermod` for shell, home, uid, password, groups changes)
- [x] 2.6 Implement `Run()` for user removal (`userdel` with optional `-r` flag)
- [x] 2.7 Implement `Check()` for dry-run support (Checker interface)
- [x] 2.8 Implement `Description()` and `Parameters()` for the Describer interface
- [x] 2.9 Add unit tests for user module in `internal/module/user/user_test.go`

## 3. Module Registration

- [x] 3.1 Add blank imports for `user` and `group` packages in the module loader (same pattern as existing modules)
- [x] 3.2 Verify both modules appear in `module.List()` output
