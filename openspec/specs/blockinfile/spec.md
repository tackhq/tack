## ADDED Requirements

### Requirement: Ensure a text block is present in a file
The `blockinfile` module SHALL ensure a block of text exists between marker lines in a target file when `state` is `present` (the default). The `path` parameter is required and specifies the target file. The `block` parameter specifies the multi-line content to manage.

#### Scenario: Block does not exist in file
- **WHEN** the file does not contain the marker lines
- **THEN** the module SHALL append the begin marker, block content, and end marker to the end of the file and return changed

#### Scenario: Block exists with same content
- **WHEN** the file contains the marker lines and the content between them matches `block`
- **THEN** the module SHALL return unchanged with no modifications

#### Scenario: Block exists with different content
- **WHEN** the file contains the marker lines but the content between them differs from `block`
- **THEN** the module SHALL replace the content between markers with the new `block` and return changed

#### Scenario: File does not exist and create is false
- **WHEN** the target file does not exist and `create` is `false` (default)
- **THEN** the module SHALL return an error indicating the file does not exist

#### Scenario: File does not exist and create is true
- **WHEN** the target file does not exist and `create` is `true`
- **THEN** the module SHALL create the file containing the markers and block content, and return changed

### Requirement: Customizable marker lines
The `blockinfile` module SHALL support a `marker` parameter that defines the format of begin and end marker lines. The default marker format SHALL be `# {mark} BOLT MANAGED BLOCK` where `{mark}` is replaced with `BEGIN` or `END`.

#### Scenario: Default markers
- **WHEN** no `marker` parameter is specified
- **THEN** the module SHALL use `# BEGIN BOLT MANAGED BLOCK` and `# END BOLT MANAGED BLOCK` as markers

#### Scenario: Custom marker with {mark} placeholder
- **WHEN** `marker` is set to `<!-- {mark} CUSTOM BLOCK -->`
- **THEN** the module SHALL use `<!-- BEGIN CUSTOM BLOCK -->` and `<!-- END CUSTOM BLOCK -->` as markers

#### Scenario: Custom marker_begin and marker_end
- **WHEN** `marker_begin` and/or `marker_end` are specified
- **THEN** the module SHALL use those values in place of `BEGIN` and `END` when expanding the `marker` format

### Requirement: Block insertion positioning
The `blockinfile` module SHALL support `insertafter` and `insertbefore` parameters to control where a new block is inserted when markers are not already present in the file.

#### Scenario: insertafter with regex pattern
- **WHEN** `insertafter` is a regex and a line matches it and markers are not present
- **THEN** the module SHALL insert the marker block after the last line matching the `insertafter` pattern

#### Scenario: insertafter EOF or not specified
- **WHEN** `insertafter` is `EOF` or not specified and markers are not present
- **THEN** the module SHALL append the marker block to the end of the file

#### Scenario: insertbefore with regex pattern
- **WHEN** `insertbefore` is a regex and a line matches it and markers are not present
- **THEN** the module SHALL insert the marker block before the first line matching the `insertbefore` pattern

#### Scenario: insertbefore BOF
- **WHEN** `insertbefore` is `BOF` and markers are not present
- **THEN** the module SHALL insert the marker block at the beginning of the file

### Requirement: Ensure a text block is absent from a file
The `blockinfile` module SHALL remove the marker lines and all content between them when `state` is `absent`.

#### Scenario: Block markers found and removed
- **WHEN** `state` is `absent` and the file contains the marker lines
- **THEN** the module SHALL remove the begin marker, end marker, and all lines between them, and return changed

#### Scenario: Block markers not found
- **WHEN** `state` is `absent` and the file does not contain the marker lines
- **THEN** the module SHALL return unchanged

#### Scenario: File does not exist for absent state
- **WHEN** `state` is `absent` and the file does not exist
- **THEN** the module SHALL return unchanged

### Requirement: Empty block removes managed content
The `blockinfile` module SHALL treat an empty `block` parameter (or missing `block`) with `state: present` as a request to keep the markers but remove the content between them.

#### Scenario: Empty block with markers present
- **WHEN** `state` is `present` and `block` is empty and markers exist with content between them
- **THEN** the module SHALL remove the content between markers (keeping markers) and return changed

#### Scenario: Empty block with markers and no content
- **WHEN** `state` is `present` and `block` is empty and markers exist with no content between them
- **THEN** the module SHALL return unchanged

### Requirement: Backup before modification
The `blockinfile` module SHALL create a timestamped backup of the file before making changes when the `backup` parameter is `true`.

#### Scenario: Backup created on change
- **WHEN** `backup` is `true` and the module would modify the file
- **THEN** the module SHALL create a backup using `module.CreateBackup()` before writing changes

#### Scenario: No backup when unchanged
- **WHEN** `backup` is `true` but no changes are needed
- **THEN** the module SHALL NOT create a backup

### Requirement: Check mode support
The `blockinfile` module SHALL implement the `Checker` interface to support dry-run execution.

#### Scenario: Check mode reports would-change
- **WHEN** check mode is invoked and the file would be modified
- **THEN** the module SHALL return a `CheckResult` with `WouldChange: true` and populate `OldContent` and `NewContent` for diff display

#### Scenario: Check mode reports no-change
- **WHEN** check mode is invoked and the file is already in the desired state
- **THEN** the module SHALL return a `CheckResult` with `WouldChange: false`

### Requirement: Module self-description
The `blockinfile` module SHALL implement the `Describer` interface, providing a description and parameter documentation for all supported parameters.

#### Scenario: Module listed in bolt list-modules
- **WHEN** a user runs `bolt list-modules`
- **THEN** the `blockinfile` module SHALL appear with its description and parameter list
