## ADDED Requirements

### Requirement: Script execution
The script plugin SHALL execute the inventory file as a subprocess with `--list` as the sole argument. The child process SHALL inherit the parent's environment variables.

#### Scenario: Script outputs valid JSON
- **WHEN** the script exits 0 and stdout contains valid JSON in Bolt inventory format
- **THEN** the plugin SHALL parse it and return a populated `*Inventory`

#### Scenario: Script outputs valid YAML
- **WHEN** the script exits 0 and stdout contains valid YAML in Bolt inventory format
- **THEN** the plugin SHALL parse it and return a populated `*Inventory`

#### Scenario: Script exits non-zero
- **WHEN** the script exits with a non-zero exit code
- **THEN** the plugin SHALL return an error that includes the exit code and stderr content

#### Scenario: Script produces no output
- **WHEN** the script exits 0 but stdout is empty
- **THEN** the plugin SHALL return an error indicating empty inventory output

### Requirement: Output format detection
The plugin SHALL detect the output format by inspecting the first non-whitespace character of stdout. If it is `{` or `[`, the output SHALL be parsed as JSON. Otherwise, it SHALL be parsed as YAML.

#### Scenario: JSON auto-detected
- **WHEN** stdout begins with `{`
- **THEN** the output SHALL be parsed as JSON

#### Scenario: YAML auto-detected
- **WHEN** stdout begins with a non-JSON character (e.g., `h`, `-`, or a letter)
- **THEN** the output SHALL be parsed as YAML

### Requirement: Script timeout
The script plugin SHALL enforce a timeout on script execution via the context. If the script does not exit within the timeout, the process SHALL be killed.

#### Scenario: Script hangs
- **WHEN** a script does not exit within the configured timeout (default 30s)
- **THEN** the process SHALL be killed and the plugin SHALL return a timeout error

### Requirement: Stderr capture
The plugin SHALL capture stderr from the script. On failure, stderr content MUST be included in the error message. On success, stderr SHALL be discarded.

#### Scenario: Script fails with diagnostic output
- **WHEN** a script exits non-zero and writes "connection refused" to stderr
- **THEN** the error message SHALL include "connection refused"
