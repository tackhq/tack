## ADDED Requirements

### Requirement: --hosts all expands entire inventory
When `--hosts all` is provided with an inventory file, bolt SHALL expand all groups, merge all top-level hosts, deduplicate, and target every resolved host.

#### Scenario: All groups expanded
- **WHEN** user runs `bolt run -i inventory.yaml --hosts all playbook.yaml`
- **THEN** every group in the inventory is expanded and all hosts are targeted

#### Scenario: Deduplication across groups
- **WHEN** inventory has host "web1" in both group "prod" and group "web"
- **THEN** "web1" appears only once in the final host list

#### Scenario: --hosts all without inventory
- **WHEN** user runs `bolt run --hosts all playbook.yaml` without `-i`
- **THEN** bolt returns error: `--hosts all requires an inventory file (-i flag)`

### Requirement: Distinct error for zero SSM tag matches
When SSM tag resolution succeeds but returns zero instances, bolt SHALL produce a specific error message identifying the tags that matched nothing.

#### Scenario: Tags match no running instances
- **WHEN** a group has SSM tags configured and tag resolution returns an empty list
- **THEN** bolt returns error mentioning "zero instances" and listing the tags used

#### Scenario: Tags match instances
- **WHEN** SSM tag resolution returns one or more instance IDs
- **THEN** execution proceeds normally with resolved hosts

### Requirement: Improved missing-hosts error message
When no hosts are resolved and the connection is non-local, the error message SHALL mention all three ways to provide hosts.

#### Scenario: No hosts specified anywhere
- **WHEN** playbook has no `hosts:` field, no `--hosts` flag, and no `-c` flag
- **THEN** error message mentions `--hosts`, playbook `hosts:` field, and `-c` flag

#### Scenario: Local connection without hosts
- **WHEN** connection type is "local" and no hosts are specified
- **THEN** no error is raised (local connections do not require hosts)
