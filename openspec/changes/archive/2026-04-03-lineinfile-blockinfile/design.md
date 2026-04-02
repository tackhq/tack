## Context

Bolt's module system provides idempotent operations via the `Module` interface. Modules like `copy` and `template` deploy entire files, but there is no way to make targeted edits to existing files. Both `lineinfile` and `blockinfile` are well-understood patterns from Ansible that users expect in a configuration management tool.

Both modules will operate entirely through the `connector.Connector` interface, reading file content via `Execute` (shell `cat`) and writing via `Upload`, making them work uniformly across local, SSH, Docker, and SSM connectors. No new connector capabilities are needed.

## Goals / Non-Goals

**Goals:**
- Provide idempotent single-line management (`lineinfile`) with regex matching
- Provide idempotent multi-line block management (`blockinfile`) with configurable markers
- Support all existing connectors without connector-level changes
- Support check mode via the `Checker` interface
- Support backup creation before modification
- Support file creation when the target does not exist (`create` parameter)

**Non-Goals:**
- Multi-line regex matching in `lineinfile` (use `blockinfile` or `template` instead)
- Preserving original file permissions across connectors that don't support it (best-effort via existing `EnsureAttributes`)
- In-place sed-style transformations or multi-match replacement within a single `lineinfile` invocation (only the last match is replaced, consistent with Ansible behavior)

## Decisions

### 1. File content manipulation via read-modify-upload pattern

Both modules will read the remote file content via `conn.Execute(ctx, "cat <path>")`, perform line-level manipulation in Go, and write the result back via `conn.Upload()`. This reuses the existing connector abstraction and avoids relying on target-side tools like `sed` or `awk`.

**Alternative considered**: Running `sed` commands on the target. Rejected because sed syntax varies across platforms (GNU vs BSD), regex escaping is error-prone, and it prevents proper idempotency checking in Go.

### 2. Line-oriented processing

Both modules split file content by `\n`, operate on the resulting slice, and rejoin with `\n`. This keeps the logic simple and predictable. Files are treated as sequences of lines.

### 3. lineinfile regex matching and replacement

When `regexp` is provided, the module scans all lines and replaces the **last** matching line with the `line` value (when `state: present`). If no match is found, the line is inserted according to `insertafter`/`insertbefore` rules, or appended to the end. This matches Ansible's behavior.

When `state: absent`, all lines matching `regexp` (or exactly matching `line` if no regexp) are removed.

**Alternative considered**: Replacing all matching lines. Rejected for `state: present` to avoid unintended duplication; accepted for `state: absent` since removing all matches is the expected behavior.

### 4. blockinfile marker format

Default markers: `# BEGIN BOLT MANAGED BLOCK` and `# END BOLT MANAGED BLOCK`. The `marker` parameter accepts a format string with `{mark}` placeholder that gets replaced with `BEGIN` or `END`. Users can also set `marker_begin` and `marker_end` for full control.

This allows markers like `<!-- BEGIN BOLT MANAGED BLOCK -->` for XML/HTML files or `// BEGIN BOLT MANAGED BLOCK` for C-style files.

### 5. Check mode implementation

Both modules implement the `Checker` interface. The `Check` method reads the remote file, performs the same line manipulation logic, and compares the result with the original content. If they differ, it returns `WouldChange`; otherwise `NoChange`. The `CheckResult` populates `OldContent` and `NewContent` for diff display.

### 6. Backup via existing helper

Both modules use `module.CreateBackup()` when the `backup` parameter is true, consistent with how `copy` and `template` handle backups.

## Risks / Trade-offs

- **Large files**: Reading entire file content into memory could be problematic for very large files. Mitigation: this is an inherent limitation of the read-modify-upload pattern and matches Ansible's behavior. Document that these modules are designed for configuration files, not large data files.
- **Concurrent edits**: If another process modifies the file between read and upload, changes could be lost. Mitigation: this is a general limitation of file management modules; no additional locking is introduced. The window is small for typical configuration management use.
- **Line ending normalization**: Files with `\r\n` line endings will be normalized to `\n` after processing. Mitigation: configuration management targets are overwhelmingly Unix-based; document this behavior.
- **Empty file handling**: When `create: true` and the file doesn't exist, the module creates the file with just the inserted content. This is correct behavior but worth noting.
