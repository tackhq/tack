## ADDED Requirements

### Requirement: Ensure a line is present in a file
The `lineinfile` module SHALL ensure a specific line exists in a target file when `state` is `present` (the default). The `path` parameter is required and specifies the target file. The `line` parameter specifies the exact line content to ensure.

#### Scenario: Line already exists in file
- **WHEN** the file contains a line exactly matching the `line` parameter
- **THEN** the module SHALL return unchanged with no modifications

#### Scenario: Line does not exist and no regexp
- **WHEN** the file does not contain the `line` and no `regexp` is specified
- **THEN** the module SHALL append the `line` to the end of the file and return changed

#### Scenario: File does not exist and create is false
- **WHEN** the target file does not exist and `create` is `false` (default)
- **THEN** the module SHALL return an error indicating the file does not exist

#### Scenario: File does not exist and create is true
- **WHEN** the target file does not exist and `create` is `true`
- **THEN** the module SHALL create the file containing only the specified `line` and return changed

### Requirement: Regex-based line matching and replacement
The `lineinfile` module SHALL support a `regexp` parameter containing a regular expression. When provided, the module searches for lines matching the pattern and replaces the last match with the `line` value.

#### Scenario: Regexp matches a line in the file
- **WHEN** `regexp` matches one or more lines in the file and `state` is `present`
- **THEN** the module SHALL replace the last matching line with the `line` value and return changed

#### Scenario: Regexp matches but line already correct
- **WHEN** `regexp` matches a line and that line already equals the `line` value
- **THEN** the module SHALL return unchanged with no modifications

#### Scenario: Regexp does not match any line
- **WHEN** `regexp` is provided but no lines match the pattern
- **THEN** the module SHALL insert the `line` according to `insertafter`/`insertbefore` rules, or append to the end if neither is specified

### Requirement: Line insertion positioning
The `lineinfile` module SHALL support `insertafter` and `insertbefore` parameters to control where a new line is inserted when it is not already present (and no regexp match is found).

#### Scenario: insertafter with regex pattern
- **WHEN** `insertafter` is a regex pattern and a line matches it
- **THEN** the module SHALL insert the new line after the last line matching the `insertafter` pattern

#### Scenario: insertafter EOF
- **WHEN** `insertafter` is `EOF` or not specified
- **THEN** the module SHALL append the new line to the end of the file

#### Scenario: insertbefore with regex pattern
- **WHEN** `insertbefore` is a regex pattern and a line matches it
- **THEN** the module SHALL insert the new line before the first line matching the `insertbefore` pattern

#### Scenario: insertbefore BOF
- **WHEN** `insertbefore` is `BOF`
- **THEN** the module SHALL insert the new line at the beginning of the file

#### Scenario: insertafter and insertbefore pattern does not match
- **WHEN** `insertafter` or `insertbefore` is a regex that does not match any line
- **THEN** the module SHALL append the line to the end of the file

### Requirement: Ensure a line is absent from a file
The `lineinfile` module SHALL remove lines from a file when `state` is `absent`.

#### Scenario: Remove line by exact match
- **WHEN** `state` is `absent` and `line` is specified without `regexp`
- **THEN** the module SHALL remove all lines exactly matching `line` and return changed

#### Scenario: Remove lines by regexp
- **WHEN** `state` is `absent` and `regexp` is specified
- **THEN** the module SHALL remove all lines matching `regexp` and return changed

#### Scenario: Line or pattern not found for removal
- **WHEN** `state` is `absent` and no lines match
- **THEN** the module SHALL return unchanged

#### Scenario: File does not exist for absent state
- **WHEN** `state` is `absent` and the file does not exist
- **THEN** the module SHALL return unchanged (nothing to remove)

### Requirement: Backup before modification
The `lineinfile` module SHALL create a timestamped backup of the file before making changes when the `backup` parameter is `true`.

#### Scenario: Backup created on change
- **WHEN** `backup` is `true` and the module would modify the file
- **THEN** the module SHALL create a backup using `module.CreateBackup()` before writing changes

#### Scenario: No backup when unchanged
- **WHEN** `backup` is `true` but no changes are needed
- **THEN** the module SHALL NOT create a backup

### Requirement: Check mode support
The `lineinfile` module SHALL implement the `Checker` interface to support dry-run execution.

#### Scenario: Check mode reports would-change
- **WHEN** check mode is invoked and the file would be modified
- **THEN** the module SHALL return a `CheckResult` with `WouldChange: true` and populate `OldContent` and `NewContent` for diff display

#### Scenario: Check mode reports no-change
- **WHEN** check mode is invoked and the file is already in the desired state
- **THEN** the module SHALL return a `CheckResult` with `WouldChange: false`

### Requirement: Module self-description
The `lineinfile` module SHALL implement the `Describer` interface, providing a description and parameter documentation for all supported parameters.

#### Scenario: Module listed in bolt list-modules
- **WHEN** a user runs `bolt list-modules`
- **THEN** the `lineinfile` module SHALL appear with its description and parameter list
