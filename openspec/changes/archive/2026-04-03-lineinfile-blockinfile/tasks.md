## 1. lineinfile Module

- [x] 1.1 Create `internal/module/lineinfile/` package with `Module` struct, `init()` registration, `Name()` returning `"lineinfile"`
- [x] 1.2 Implement core line manipulation helpers: `readRemoteFile`, `findLastMatch`, `insertAtPosition`, and `removeLine` functions
- [x] 1.3 Implement `Run()` for `state: present` -- exact line match (append if missing, no-op if present)
- [x] 1.4 Implement `Run()` for `state: present` with `regexp` -- replace last matching line or insert if no match
- [x] 1.5 Implement `insertafter` and `insertbefore` positioning logic (EOF, BOF, regex patterns)
- [x] 1.6 Implement `Run()` for `state: absent` -- remove all matching lines by exact match or regexp
- [x] 1.7 Implement `create` parameter support -- create file if missing when `create: true`, error when `false`
- [x] 1.8 Implement `backup` parameter support using `module.CreateBackup()`
- [x] 1.9 Implement `Checker` interface (`Check` method) with `OldContent`/`NewContent` population for diff display
- [x] 1.10 Implement `Describer` interface (`Description` and `Parameters` methods)
- [x] 1.11 Write unit tests for lineinfile: present/absent states, regexp matching, insertion positioning, create, backup, check mode, edge cases (empty file, missing file)

## 2. blockinfile Module

- [x] 2.1 Create `internal/module/blockinfile/` package with `Module` struct, `init()` registration, `Name()` returning `"blockinfile"`
- [x] 2.2 Implement marker handling: default markers, `marker` with `{mark}` placeholder expansion, `marker_begin`/`marker_end` overrides
- [x] 2.3 Implement `Run()` for `state: present` -- find existing markers and replace content between them, or insert new marker block
- [x] 2.4 Implement `insertafter` and `insertbefore` positioning for new block insertion (EOF, BOF, regex patterns)
- [x] 2.5 Implement `Run()` for `state: absent` -- remove marker lines and all content between them
- [x] 2.6 Implement empty block handling -- keep markers but clear content between them
- [x] 2.7 Implement `create` and `backup` parameter support
- [x] 2.8 Implement `Checker` interface (`Check` method) with `OldContent`/`NewContent` population for diff display
- [x] 2.9 Implement `Describer` interface (`Description` and `Parameters` methods)
- [x] 2.10 Write unit tests for blockinfile: present/absent states, custom markers, insertion positioning, empty block, create, backup, check mode, edge cases

## 3. Integration and Registration

- [x] 3.1 Add module imports to ensure `init()` registration (add blank imports in the appropriate location if needed)
- [x] 3.2 Add integration test playbook exercising both modules across lineinfile and blockinfile scenarios
- [x] 3.3 Verify `make build`, `make test`, and `make lint` pass cleanly
