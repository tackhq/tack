## ADDED Requirements

### Requirement: Execute external script as inventory source
The system SHALL execute an external script or binary specified via the `-i` flag and parse its stdout as JSON inventory data. The script MUST be an executable file (has execute permission). The system SHALL pass the script's stderr through to bolt's stderr stream for debugging purposes.

#### Scenario: Script produces valid inventory JSON
- **WHEN** the `-i` flag points to an executable file that outputs valid JSON inventory to stdout
- **THEN** the system executes the script, parses the JSON output into the Inventory struct, and uses it for the playbook run

#### Scenario: Script exits with non-zero code
- **WHEN** the `-i` flag points to an executable script that exits with a non-zero exit code
- **THEN** the system SHALL return an error indicating the script failed with the exit code and include any stderr output in the error message

#### Scenario: Script produces invalid JSON
- **WHEN** the `-i` flag points to an executable script that outputs invalid JSON to stdout
- **THEN** the system SHALL return a parse error indicating the script output could not be decoded as inventory JSON

### Requirement: Script inherits environment variables
The system SHALL execute inventory scripts with the current process environment variables inherited, allowing scripts to use `BOLT_*` environment variables and AWS credentials for their discovery logic.

#### Scenario: Script accesses environment variables
- **WHEN** an inventory script reads environment variables during execution
- **THEN** the script SHALL have access to all environment variables from the parent bolt process

### Requirement: Script execution respects context cancellation
The system SHALL execute inventory scripts using the command context, so that global timeouts and cancellation signals terminate the script process.

#### Scenario: Script execution cancelled by timeout
- **WHEN** an inventory script is running and the context is cancelled (e.g., timeout)
- **THEN** the system SHALL terminate the script process and return a context cancellation error

### Requirement: Standard JSON inventory schema
The JSON output from inventory scripts MUST conform to the standard inventory schema with top-level `hosts` and `groups` objects. The `hosts` object maps host names to entries with optional `ssh` config and `vars`. The `groups` object maps group names to entries with optional `connection`, `ssh`, `ssm`, `hosts` list, and `vars`.

#### Scenario: Script outputs hosts with SSH config
- **WHEN** a script outputs JSON with hosts containing `ssh` configuration (host, user, port, key)
- **THEN** the system SHALL populate the corresponding `HostEntry.SSH` fields and make them available for connection setup

#### Scenario: Script outputs groups with vars
- **WHEN** a script outputs JSON with groups containing `vars` maps
- **THEN** the system SHALL populate the corresponding `GroupEntry.Vars` fields and make them available for template interpolation
