## ADDED Requirements

### Requirement: Inventory list subcommand
The system SHALL provide a `tack inventory --list` subcommand (or `tack inventory list`) that loads the inventory from the specified `-i` source(s) and prints the resolved result as JSON to stdout.

#### Scenario: List static inventory
- **WHEN** user runs `tack inventory --list -i hosts.yml`
- **THEN** the system SHALL print the parsed inventory as JSON with hosts and groups

#### Scenario: List dynamic inventory
- **WHEN** user runs `tack inventory --list -i ./ec2-inventory.py`
- **THEN** the system SHALL execute the script, resolve the inventory, and print the result as JSON

#### Scenario: List plugin inventory
- **WHEN** user runs `tack inventory --list -i ec2.yml` where the file contains `plugin: ec2`
- **THEN** the system SHALL load via the EC2 plugin and print the resolved inventory as JSON

#### Scenario: No inventory specified
- **WHEN** user runs `tack inventory --list` without `-i`
- **THEN** the system SHALL print an error indicating that an inventory source is required

### Requirement: Inventory list output format
The JSON output SHALL mirror the Tack inventory structure with `hosts` and `groups` top-level keys. Host entries SHALL include their vars, SSH config, and any other metadata. Group entries SHALL include their host lists, vars, and connection settings.

#### Scenario: Output includes host vars
- **WHEN** the inventory contains a host with vars `{region: us-east-1}`
- **THEN** the JSON output SHALL include those vars under the host's entry

#### Scenario: Output includes group details
- **WHEN** the inventory contains a group with hosts, connection type, and vars
- **THEN** the JSON output SHALL include all group fields

### Requirement: Inventory host detail
The system SHALL support `tack inventory --host <name>` to print details for a single host, including all resolved vars (merged from host-level and group-level).

#### Scenario: Show single host
- **WHEN** user runs `tack inventory --host web1 -i hosts.yml`
- **THEN** the system SHALL print the host's vars, SSH config, and group memberships as JSON

#### Scenario: Unknown host
- **WHEN** user runs `tack inventory --host unknown -i hosts.yml` and "unknown" is not in the inventory
- **THEN** the system SHALL print an error indicating the host was not found
