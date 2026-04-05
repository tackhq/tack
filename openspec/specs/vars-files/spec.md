## ADDED Requirements

### Requirement: Load variables from external files
Plays SHALL support a `vars_files:` directive that accepts a list of YAML file paths. Each file SHALL be parsed as a flat `key: value` YAML map and merged into the play's variable scope.

#### Scenario: Single vars file
- **WHEN** a play specifies `vars_files: ["vars/common.yaml"]` and the file contains `app_port: 8080`
- **THEN** the variable `app_port` SHALL be available in tasks with value `8080`

#### Scenario: Multiple vars files with override
- **WHEN** a play specifies `vars_files: ["vars/defaults.yaml", "vars/prod.yaml"]` and both define `db_host`
- **THEN** the value from `vars/prod.yaml` SHALL take precedence (last file wins)

### Requirement: File paths relative to playbook
File paths in `vars_files:` SHALL be resolved relative to the playbook file's directory.

#### Scenario: Relative path resolution
- **WHEN** the playbook is at `/deploy/playbook.yaml` and vars_files specifies `vars/prod.yaml`
- **THEN** the executor SHALL load `/deploy/vars/prod.yaml`

### Requirement: Variable interpolation in paths
File paths SHALL support `{{ variable }}` interpolation using play-level vars and extra-vars.

#### Scenario: Dynamic file selection
- **WHEN** a play has `vars: {env: prod}` and `vars_files: ["vars/{{ env }}.yaml"]`
- **THEN** the executor SHALL load `vars/prod.yaml`

#### Scenario: Extra-vars in path
- **WHEN** the user passes `--extra-vars env=staging` and vars_files specifies `vars/{{ env }}.yaml`
- **THEN** the executor SHALL load `vars/staging.yaml`

### Requirement: Missing file handling
The executor SHALL return an error if a vars file does not exist, unless the path is marked as optional with a `?` prefix.

#### Scenario: Missing required file
- **WHEN** vars_files specifies `vars/missing.yaml` and the file does not exist
- **THEN** the executor SHALL return an error with the file path in the message

#### Scenario: Missing optional file
- **WHEN** vars_files specifies `?vars/local-overrides.yaml` and the file does not exist
- **THEN** the executor SHALL skip the file without error

### Requirement: Variable precedence
Variables from `vars_files:` SHALL have higher precedence than play-level `vars:` but lower precedence than inventory host/group vars.

#### Scenario: vars_files overrides play vars
- **WHEN** play vars defines `port: 80` and vars_files defines `port: 8080`
- **THEN** `port` SHALL resolve to `8080`

#### Scenario: Inventory vars overrides vars_files
- **WHEN** vars_files defines `port: 8080` and inventory host vars defines `port: 9090`
- **THEN** `port` SHALL resolve to `9090`

### Requirement: Playbook validation
The `tack validate` command SHALL check that vars_files paths are syntactically valid but SHALL NOT require the files to exist at validation time (they may be generated or environment-specific).

#### Scenario: Validate with vars_files
- **WHEN** running `tack validate playbook.yaml` and the playbook has `vars_files: ["vars/{{ env }}.yaml"]`
- **THEN** validation SHALL pass (file existence not checked)
