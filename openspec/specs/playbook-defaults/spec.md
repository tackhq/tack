## ADDED Requirements

### Requirement: Playbook supports a top-level mapping format with defaults

A playbook file SHALL be parseable in two interchangeable formats:

1. **Sequence format** (existing): the YAML root is a sequence of plays.
2. **Mapping format** (new): the YAML root is a mapping containing a required `plays:` sequence and optional default fields (`hosts`, `connection`, `sudo`, `vars`) that are inherited by every play in `plays:`.

The parser SHALL detect the format from the YAML root node kind and produce an equivalent `Playbook` value either way.

#### Scenario: Sequence format continues to parse unchanged
- **WHEN** a playbook file's root node is a YAML sequence
- **THEN** each element is parsed as a play exactly as before, with no implicit defaults applied

#### Scenario: Mapping format with `plays:` parses
- **WHEN** a playbook file's root node is a mapping containing a `plays:` sequence
- **THEN** each element under `plays:` is parsed as a play and the mapping's `hosts`, `connection`, `sudo`, and `vars` fields are recorded as playbook-level defaults

#### Scenario: Mapping format without `plays:` is a parse error
- **WHEN** a playbook file's root node is a mapping with no `plays:` key
- **THEN** the parser returns an error indicating that `plays:` is required in the mapping format

### Requirement: Plays inherit playbook-level defaults when not set

When the mapping format declares `hosts`, `connection`, or `sudo` at the playbook level, plays under `plays:` SHALL inherit those values unless the play sets the same field. Play-level values always win over playbook-level defaults; defaults are not merged element-wise into play-level lists or maps for these scalar/list fields.

For `vars`, playbook-level keys SHALL be merged into each play's vars map; play-level keys override playbook-level keys on conflict.

#### Scenario: Play omits hosts and inherits from playbook
- **WHEN** the playbook declares `hosts: webservers` and a play omits `hosts:`
- **THEN** the play's effective `Hosts` is `["webservers"]` after parsing

#### Scenario: Play overrides playbook-level hosts
- **WHEN** the playbook declares `hosts: webservers` and a play declares `hosts: dbservers`
- **THEN** the play's effective `Hosts` is `["dbservers"]`; the playbook default is ignored for that play

#### Scenario: Connection and sudo inherit independently
- **WHEN** the playbook declares `connection: ssh` and `sudo: true`, and a play omits both
- **THEN** the play's effective `Connection` is `"ssh"` and `Sudo` is `true`

#### Scenario: Vars are merged with play-level precedence
- **WHEN** the playbook declares `vars: {env: prod, tier: web}` and a play declares `vars: {tier: api}`
- **THEN** the play's effective vars are `{env: prod, tier: api}`

#### Scenario: Empty playbook defaults are not applied
- **WHEN** the playbook mapping omits a default field (e.g. no `connection:` at the playbook level)
- **THEN** plays without their own `connection:` retain the existing zero-value behavior, identical to the sequence format

### Requirement: Validation accepts hosts declared at either level

The `tack validate` command and the parser's structural validation SHALL accept a playbook as valid when each play has hosts available, whether declared on the play itself or inherited from the playbook-level `hosts:`. A play with no hosts at either level SHALL fail validation with a clear message indicating where to declare hosts.

#### Scenario: Play with playbook-level hosts passes validation
- **WHEN** the mapping format declares `hosts: webservers` and a play has no `hosts:` field
- **THEN** validation succeeds for that play

#### Scenario: Play missing hosts at both levels fails validation
- **WHEN** neither the playbook nor a play declares `hosts:`
- **THEN** validation fails with an error referencing both the play and the option to set playbook-level `hosts:`
