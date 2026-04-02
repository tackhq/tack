## Context

Bolt's plan phase already captures file content diffs internally. The `CheckDeployFile` helper (`internal/module/fileutil.go`) fetches remote file content via `cat`, computes SHA256 checksums, and populates `CheckResult.OldContent`/`NewContent`. The executor threads these into `PlannedTask` structs, and `DisplayPlan` renders colored unified diffs — but only when `--verbose` is set.

The problem: `--verbose` is semantically overloaded. Users expect it to mean "more logging," not "show file diffs." The diff display is effectively hidden. Additionally, the current diff algorithm is naive (two-pointer, no LCS), there are no file path headers, new/deleted files show only checksums, large/binary files aren't handled, and JSON output omits all diff data.

## Goals / Non-Goals

**Goals:**
- Dedicated `--diff` flag decoupled from `--verbose`
- Proper unified diff output with file path headers and context windowing
- Safe handling of binary and large files
- JSON output enriched with diff data for CI tooling
- Gate expensive remote `cat` behind diff/verbose to avoid unnecessary network I/O

**Non-Goals:**
- Side-by-side diff display (terminal width issues)
- Diff for non-file modules (apt, brew, systemd, command)
- Interactive diff review or patch application
- Diff persistence/caching across runs

## Decisions

### 1. Dedicated `--diff` flag alongside `--verbose`

Add `--diff` as a persistent bool flag on root command. Diff display triggers on `o.diff || o.verbose` to preserve backward compatibility. `--verbose` continues to show diffs (existing behavior) plus any future verbose logging.

**Alternative considered:** Replacing `--verbose` entirely with `--diff`. Rejected because `--verbose` may gain non-diff uses later, and changing its behavior is a breaking change for scripts.

### 2. Use `go-difflib` for diff algorithm

Replace the custom two-pointer `unifiedDiff()` with `github.com/pmezard/go-difflib/difflib`. This library implements Myers diff and produces standard unified diff format. It's already an indirect dependency via `testify`.

**Alternative considered:** Implementing Myers diff from scratch. Rejected — unnecessary complexity for a solved problem with a well-tested library already in the dep tree.

### 3. Context-window limiting (±3 lines)

Use `difflib.UnifiedDiff` with `Context: 3` to show only 3 lines around each change. Long unchanged sections are collapsed with `@@` markers, matching standard `diff -u` output.

### 4. Thread diff flag via `Output` struct

Add a `diff` bool field to the `Output` struct (and `JSONEmitter`). The CLI constructs Output with the flag value. This avoids threading it through executor/module layers — the display layer controls what to show, not the execution layer.

**For gating the remote `cat`:** Add a `DiffEnabled` field to a new `CheckOptions` struct passed to `CheckDeployFile`. When false, skip fetching remote content (checksums are still computed via the existing `sha256sum` command). This avoids adding the flag to the `Checker` interface.

### 5. Binary detection via null byte scan

Before rendering a diff, scan the first 8KB of both old and new content for null bytes (`\x00`). If found, display "Binary files differ" with checksums only. This matches `git diff` behavior.

### 6. Size threshold of 64KB

If remote file content exceeds 64KB, skip diff rendering and show checksums only with a "(file too large for diff)" note. The `cat` fetch is still gated by the diff flag — this threshold applies to already-fetched content to keep terminal output manageable.

### 7. JSON output enrichment

Add `old_checksum`, `new_checksum` to all `plan_task` events when available. Add `old_content`, `new_content` only when `--diff` is active, to avoid bloating output. This is additive — no existing fields change.

## Risks / Trade-offs

- **[Performance] Remote `cat` on many hosts with `--diff`** → Mitigated by size threshold and by only fetching when diff flag is set. The `cat` runs during the read-only check phase, not apply.
- **[Output noise] Large diffs flooding terminal** → Mitigated by context windowing (±3 lines) and size threshold (64KB). Users can combine with `--output json` for machine processing.
- **[Dependency] Promoting go-difflib to direct dep** → Minimal risk — it's already in `go.sum` via testify, well-maintained, and has no transitive dependencies.
- **[Backward compatibility] `--verbose` still shows diffs** → This is intentional. Existing scripts using `--verbose` won't break. New users get the more explicit `--diff` flag.
