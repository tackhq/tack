## ADDED Requirements

### Requirement: Group creation
The `group` module SHALL create a system group when `state` is `present` (default) and the group does not exist. The module SHALL use `groupadd` to create the group.

#### Scenario: Create group with defaults
- **WHEN** `group` module is run with `name: deploy` and group `deploy` does not exist
- **THEN** the module SHALL execute `groupadd deploy` and return `changed: true`

#### Scenario: Create group with gid
- **WHEN** `group` module is run with `name: deploy`, `gid: 1500` and group `deploy` does not exist
- **THEN** the module SHALL execute `groupadd -g 1500 deploy` and return `changed: true`

#### Scenario: Create system group
- **WHEN** `group` module is run with `name: deploy`, `system: true` and group `deploy` does not exist
- **THEN** the module SHALL execute `groupadd -r deploy` and return `changed: true`

### Requirement: Group idempotency
The `group` module SHALL NOT make changes when the group already exists and all specified attributes match the current state.

#### Scenario: Group already exists with matching attributes
- **WHEN** `group` module is run with `name: deploy`, `gid: 1500` and group `deploy` exists with gid `1500`
- **THEN** the module SHALL return `changed: false` without executing any commands

#### Scenario: Group exists but gid differs
- **WHEN** `group` module is run with `name: deploy`, `gid: 1600` and group `deploy` exists with gid `1500`
- **THEN** the module SHALL execute `groupmod -g 1600 deploy` and return `changed: true`

### Requirement: Group removal
The `group` module SHALL remove a group when `state` is `absent` and the group exists.

#### Scenario: Remove existing group
- **WHEN** `group` module is run with `name: deploy`, `state: absent` and the group exists
- **THEN** the module SHALL execute `groupdel deploy` and return `changed: true`

#### Scenario: Remove non-existent group
- **WHEN** `group` module is run with `name: deploy`, `state: absent` and the group does not exist
- **THEN** the module SHALL return `changed: false`

### Requirement: Group check mode
The `group` module SHALL implement the `Checker` interface for dry-run support.

#### Scenario: Check mode detects needed creation
- **WHEN** check mode is run with `name: deploy` and group `deploy` does not exist
- **THEN** the module SHALL return `would_change: true` with message indicating group would be created

#### Scenario: Check mode detects no change needed
- **WHEN** check mode is run with `name: deploy` and group `deploy` exists with matching attributes
- **THEN** the module SHALL return `would_change: false`

### Requirement: Group parameter validation
The `group` module SHALL validate required parameters and return descriptive errors.

#### Scenario: Missing name parameter
- **WHEN** `group` module is run without the `name` parameter
- **THEN** the module SHALL return an error indicating `name` is required

#### Scenario: Invalid state parameter
- **WHEN** `group` module is run with `state: invalid`
- **THEN** the module SHALL return an error indicating valid states are `present` and `absent`
