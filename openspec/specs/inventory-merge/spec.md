## ADDED Requirements

### Requirement: Multiple inventory sources
The system SHALL accept multiple `-i` flags on the CLI. Each source SHALL be loaded independently (static, script, or plugin) and the results merged into a single `*Inventory`.

#### Scenario: Two static files merged
- **WHEN** user runs `tack run playbook.yml -i hosts1.yml -i hosts2.yml`
- **THEN** the system SHALL load both files and merge their inventories

#### Scenario: Mixed source types
- **WHEN** user runs `tack run playbook.yml -i ./script.sh -i overrides.yml`
- **THEN** the system SHALL execute the script for the first source, parse YAML for the second, and merge results

### Requirement: Host merge semantics
When the same host name appears in multiple sources, the later source SHALL override scalar fields (vars values, SSH config fields). Hosts unique to each source SHALL all be included in the merged result.

#### Scenario: Later source overrides host vars
- **WHEN** source 1 defines host `web1` with `{region: us-east-1}` and source 2 defines `web1` with `{region: eu-west-1}`
- **THEN** the merged inventory SHALL have `web1` with `{region: eu-west-1}`

#### Scenario: Unique hosts from both sources
- **WHEN** source 1 defines `web1` and source 2 defines `db1`
- **THEN** the merged inventory SHALL contain both `web1` and `db1`

### Requirement: Group merge semantics
When the same group name appears in multiple sources, host lists SHALL be combined (union, deduplicated). Group vars SHALL be deep-merged with later sources winning on key conflicts. Connection and SSH/SSM config from the later source SHALL override the earlier.

#### Scenario: Group host lists merged
- **WHEN** source 1 defines group `web` with hosts `[web1]` and source 2 defines `web` with hosts `[web2]`
- **THEN** the merged group `web` SHALL contain `[web1, web2]`

#### Scenario: Group vars deep-merged
- **WHEN** source 1 defines group vars `{port: 8080, env: staging}` and source 2 defines `{env: prod, region: us-east-1}`
- **THEN** the merged vars SHALL be `{port: 8080, env: prod, region: us-east-1}`

#### Scenario: Deduplicated host lists
- **WHEN** both sources include `web1` in the same group
- **THEN** the merged group SHALL contain `web1` only once

### Requirement: Merge order
Sources SHALL be merged in the order they appear on the command line. The first `-i` is the base, each subsequent `-i` overlays onto it.

#### Scenario: Three sources in order
- **WHEN** user provides `-i base.yml -i overrides.yml -i local.yml`
- **THEN** `base.yml` SHALL be loaded first, `overrides.yml` merged on top, then `local.yml` merged last
