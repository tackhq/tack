## 1. CLI Flag & Output Struct

- [x] 1.1 Add `--diff` persistent bool flag to root command in `cmd/bolt/main.go`
- [x] 1.2 Add `diff` bool field to `Output` struct in `internal/output/output.go` and accept it in constructor
- [x] 1.3 Add `diff` bool field to `JSONEmitter` struct in `internal/output/json.go` and accept it in constructor
- [x] 1.4 Wire `showDiff` flag value through to `Output`/`JSONEmitter` construction in the run command

## 2. Diff Display Gate

- [x] 2.1 Change diff display condition in `DisplayPlan` from `o.verbose` to `o.verbose || o.diff` (`output.go:283`)
- [x] 2.2 Add file path headers (`--- /path` / `+++ /path`) to diff blocks — extract dest path from task params
- [x] 2.3 Handle new file display: show `--- /dev/null` / `+++ /path` with all lines as `+` additions (currently only shows checksum at line 299)
- [x] 2.4 Handle deleted file display: show `--- /path` / `+++ /dev/null` with all lines as `-` removals

## 3. Diff Algorithm Upgrade

- [x] 3.1 Promote `github.com/pmezard/go-difflib` from indirect to direct dependency (`go get`)
- [x] 3.2 Replace custom `unifiedDiff()` and `findNextMatch()` with `difflib.UnifiedDiff` using `Context: 3`
- [x] 3.3 Parse difflib output and apply color coding (green for `+`, red for `-`, gray for context, cyan for `@@` markers)

## 4. Binary & Large File Handling

- [x] 4.1 Add `isBinary(content string) bool` helper that scans first 8KB for null bytes
- [x] 4.2 Add size threshold check (64KB) before rendering diff content
- [x] 4.3 Integrate binary/size checks into `DisplayPlan` — show "Binary files differ" or "(file too large for diff)" with checksums

## 5. Gate Remote Content Fetch

- [x] 5.1 Add `DiffEnabled bool` field to a `CheckOptions` struct in `internal/module/fileutil.go`
- [x] 5.2 Update `CheckDeployFile` to skip the remote `cat` fetch when `DiffEnabled` is false (checksums still computed via `sha256sum`)
- [x] 5.3 Thread `DiffEnabled` from executor's check phase based on `--diff` or `--verbose` flag state
- [x] 5.4 Update copy and template module `Check()` methods to pass `CheckOptions` through

## 6. JSON Output Enrichment

- [x] 6.1 Add `old_checksum` and `new_checksum` fields to `plan_task` JSON events when available
- [x] 6.2 Add `old_content` and `new_content` fields to `plan_task` JSON events only when `--diff` is active

## 7. Tests

- [x] 7.1 Test `--diff` flag is accepted and threads through to Output struct
- [x] 7.2 Test diff display triggers on `--diff` alone (not requiring `--verbose`)
- [x] 7.3 Test diff output includes file path headers
- [x] 7.4 Test new file shows all-additions diff
- [x] 7.5 Test binary file detection shows "Binary files differ"
- [x] 7.6 Test large file shows "(file too large for diff)" with checksums
- [x] 7.7 Test JSON output includes checksums and conditional content fields
- [x] 7.8 Test `--verbose` still shows diffs (backward compatibility)
