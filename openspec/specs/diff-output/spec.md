## ADDED Requirements

### Requirement: Diff display in plan output
When `--diff` or `--verbose` is active and a file-changing task has content differences, Tack SHALL display a unified diff of the old and new file content during the plan phase.

#### Scenario: Diff shown for changed file
- **WHEN** `--diff` is specified and a `copy` or `template` task would change a remote file
- **THEN** the plan output SHALL show a unified diff with `---` (old) and `+++` (new) headers containing the remote file path, followed by hunks with `@@` markers and ±3 lines of context around each change

#### Scenario: Diff shown via --verbose
- **WHEN** `--verbose` is specified (without `--diff`) and a task has content differences
- **THEN** the plan output SHALL show the same unified diff as `--diff` (backward compatible)

#### Scenario: No diff flag
- **WHEN** neither `--diff` nor `--verbose` is specified and a task has content differences
- **THEN** the plan output SHALL show only old/new checksums (current behavior)

### Requirement: Diff file path headers
Each diff block SHALL include file path headers identifying the target file.

#### Scenario: Existing file modified
- **WHEN** a diff is displayed for an existing remote file at `/etc/nginx/nginx.conf`
- **THEN** the diff SHALL begin with `--- /etc/nginx/nginx.conf` and `+++ /etc/nginx/nginx.conf`

### Requirement: New file diff display
When a task creates a new file, the diff SHALL show all new content as additions.

#### Scenario: New file creation
- **WHEN** `--diff` is active and a `copy` task creates a file that does not exist on the target
- **THEN** the diff SHALL show `--- /dev/null` and `+++ /path/to/file` with all content lines prefixed with `+`

### Requirement: Deleted file diff display
When a task removes a file, the diff SHALL show all old content as removals.

#### Scenario: File deletion
- **WHEN** `--diff` is active and a `file` task with `state: absent` removes an existing file
- **THEN** the diff SHALL show `--- /path/to/file` and `+++ /dev/null` with all content lines prefixed with `-`

### Requirement: Context-window limiting
Diffs SHALL show only a configurable number of context lines around each change, collapsing unchanged sections.

#### Scenario: Large file with small change
- **WHEN** `--diff` is active and a 200-line file has a 1-line change at line 100
- **THEN** the diff SHALL show ±3 lines of context around the change with `@@` hunk markers, not all 200 lines

### Requirement: Binary file detection
Tack SHALL detect binary files and skip content diff rendering.

#### Scenario: Binary file detected
- **WHEN** `--diff` is active and the file content (old or new) contains null bytes in the first 8KB
- **THEN** the plan output SHALL show "Binary files differ" with old/new checksums instead of a content diff

### Requirement: Large file threshold
Tack SHALL skip content diff rendering for files exceeding a size threshold.

#### Scenario: File exceeds 64KB
- **WHEN** `--diff` is active and the file content exceeds 64KB
- **THEN** the plan output SHALL show old/new checksums with a "(file too large for diff)" note instead of a content diff

### Requirement: Gate remote content fetch
Tack SHALL only fetch remote file content (for diff purposes) when `--diff` or `--verbose` is active.

#### Scenario: No diff flag skips content fetch
- **WHEN** neither `--diff` nor `--verbose` is specified
- **THEN** the check phase SHALL compute remote checksums but SHALL NOT fetch remote file content via `cat`

#### Scenario: Diff flag enables content fetch
- **WHEN** `--diff` is specified
- **THEN** the check phase SHALL fetch remote file content for comparison
