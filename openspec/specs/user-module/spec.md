## ADDED Requirements

### Requirement: User creation
The `user` module SHALL create a system user when `state` is `present` (default) and the user does not exist. The module SHALL use `useradd` to create the user with the specified parameters.

#### Scenario: Create user with defaults
- **WHEN** `user` module is run with `name: deploy` and the user `deploy` does not exist
- **THEN** the module SHALL execute `useradd deploy` and return `changed: true`

#### Scenario: Create user with all options
- **WHEN** `user` module is run with `name: app`, `shell: /bin/bash`, `home: /opt/app`, `uid: 1500`, `groups: [docker, wheel]`, `system: true`
- **THEN** the module SHALL execute `useradd` with flags `-s /bin/bash -d /opt/app -u 1500 -G docker,wheel -r app` and return `changed: true`

#### Scenario: Create user with password
- **WHEN** `user` module is run with `name: deploy` and `password: $6$rounds=...` and the user does not exist
- **THEN** the module SHALL execute `useradd -p '<hash>' deploy` and return `changed: true`

### Requirement: User idempotency
The `user` module SHALL NOT make changes when the user already exists and all specified attributes match the current state.

#### Scenario: User already exists with matching attributes
- **WHEN** `user` module is run with `name: deploy`, `shell: /bin/bash` and user `deploy` exists with shell `/bin/bash`
- **THEN** the module SHALL return `changed: false` without executing any commands

#### Scenario: User exists but attributes differ
- **WHEN** `user` module is run with `name: deploy`, `shell: /bin/zsh` and user `deploy` exists with shell `/bin/bash`
- **THEN** the module SHALL execute `usermod -s /bin/zsh deploy` and return `changed: true`

### Requirement: User group management
The `user` module SHALL manage supplementary group membership when the `groups` parameter is provided.

#### Scenario: Add user to supplementary groups
- **WHEN** `user` module is run with `name: deploy`, `groups: [docker, wheel]` and user `deploy` exists but is not in those groups
- **THEN** the module SHALL execute `usermod -aG docker,wheel deploy` and return `changed: true`

#### Scenario: User already in specified groups
- **WHEN** `user` module is run with `name: deploy`, `groups: [docker]` and user `deploy` is already in group `docker`
- **THEN** the module SHALL return `changed: false`

### Requirement: User removal
The `user` module SHALL remove a user when `state` is `absent` and the user exists.

#### Scenario: Remove user without home directory
- **WHEN** `user` module is run with `name: deploy`, `state: absent` and the user exists
- **THEN** the module SHALL execute `userdel deploy` and return `changed: true`

#### Scenario: Remove user with home directory
- **WHEN** `user` module is run with `name: deploy`, `state: absent`, `remove: true` and the user exists
- **THEN** the module SHALL execute `userdel -r deploy` and return `changed: true`

#### Scenario: Remove non-existent user
- **WHEN** `user` module is run with `name: deploy`, `state: absent` and the user does not exist
- **THEN** the module SHALL return `changed: false`

### Requirement: User check mode
The `user` module SHALL implement the `Checker` interface for dry-run support.

#### Scenario: Check mode detects needed creation
- **WHEN** check mode is run with `name: deploy` and user `deploy` does not exist
- **THEN** the module SHALL return `would_change: true` with message indicating user would be created

#### Scenario: Check mode detects no change needed
- **WHEN** check mode is run with `name: deploy` and user `deploy` exists with matching attributes
- **THEN** the module SHALL return `would_change: false`

### Requirement: User parameter validation
The `user` module SHALL validate required parameters and return descriptive errors.

#### Scenario: Missing name parameter
- **WHEN** `user` module is run without the `name` parameter
- **THEN** the module SHALL return an error indicating `name` is required

#### Scenario: Invalid state parameter
- **WHEN** `user` module is run with `state: invalid`
- **THEN** the module SHALL return an error indicating valid states are `present` and `absent`
