## ADDED Requirements

### Requirement: Sudo enables root execution
The Docker connector SHALL execute commands as `root` when `SetSudo(true, _)` is called, by passing `-u root` to `docker exec`.

#### Scenario: Sudo enabled
- **WHEN** `SetSudo(true, "")` is called on the Docker connector
- **THEN** subsequent `Execute()` calls SHALL use `docker exec -u root`

#### Scenario: Sudo disabled after being enabled
- **WHEN** `SetSudo(true, "")` is called, then `SetSudo(false, "")` is called
- **THEN** subsequent `Execute()` calls SHALL revert to the originally configured user

### Requirement: Default user preserved
The Docker connector SHALL preserve the originally configured user (from `WithUser()` option or container default) and restore it when sudo is disabled.

#### Scenario: Custom user with sudo toggle
- **WHEN** the connector is created with `WithUser("appuser")` and `SetSudo(true, "")` is called
- **THEN** commands SHALL run as `root`
- **AND WHEN** `SetSudo(false, "")` is called
- **THEN** commands SHALL run as `appuser`

#### Scenario: No custom user with sudo toggle
- **WHEN** no user is configured and `SetSudo(true, "")` is called
- **THEN** commands SHALL run as `root`
- **AND WHEN** `SetSudo(false, "")` is called
- **THEN** commands SHALL run with no `-u` flag (container default)

### Requirement: Password parameter accepted but unused
The Docker connector SHALL accept the password parameter in `SetSudo()` without error but SHALL NOT use it, as Docker privilege escalation does not require passwords.

#### Scenario: Password provided
- **WHEN** `SetSudo(true, "mypassword")` is called
- **THEN** the connector SHALL not error and SHALL ignore the password
