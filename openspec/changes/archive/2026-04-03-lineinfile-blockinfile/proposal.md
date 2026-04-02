## Why

Bolt currently supports full file deployment via `copy` and `template` modules, but lacks the ability to make surgical edits to existing files. Common configuration management tasks -- adding a line to `/etc/hosts`, ensuring a kernel parameter in `sysctl.conf`, managing a block of custom rules in a config file -- require reading, modifying, and rewriting entire files or resorting to raw `command` module shell scripting. This gap forces users into non-idempotent workarounds and makes simple configuration tweaks unnecessarily complex.

## What Changes

- Add a `lineinfile` module that ensures a specific line is present or absent in a file, with optional regex matching for line replacement
- Add a `blockinfile` module that manages a block of multi-line text between marker lines, enabling idempotent insertion, update, and removal of text blocks
- Both modules support backup creation, file creation (when the target file does not yet exist), and check mode
- Both modules operate via the connector interface (local, SSH, Docker, SSM) using shell commands to read/write files on the target

## Capabilities

### New Capabilities
- `lineinfile`: Ensures a single line is present or absent in a file, with regex matching, insertafter/insertbefore positioning, and idempotent state management
- `blockinfile`: Manages a marked block of multi-line text in a file, with customizable begin/end markers, insertafter/insertbefore positioning, and idempotent state management

### Modified Capabilities

None.

## Impact

- **New code**: `internal/module/lineinfile/` and `internal/module/blockinfile/` packages, each registering via `init()` following the standard module pattern
- **Module registry**: Two new modules (`lineinfile`, `blockinfile`) added to the global registry
- **Existing helpers**: Leverages `module.CreateBackup()`, `module.RequireString()`, `module.GetString()`, `module.GetBool()`, and connector `Execute`/`Upload` methods -- no changes to existing code
- **Dependencies**: No new external dependencies; uses standard library `regexp` and existing connector/module packages
- **Documentation**: New module entries for `bolt list-modules` output (via `Describer` interface)
