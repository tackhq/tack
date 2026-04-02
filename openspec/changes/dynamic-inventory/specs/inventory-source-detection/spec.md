## ADDED Requirements

### Requirement: Auto-detect inventory source type from flag value
The system SHALL automatically detect the inventory source type based on the value of the `-i` flag. Detection SHALL follow this precedence: (1) `ec2://` prefix triggers the EC2 plugin, (2) `http://` or `https://` prefix triggers the HTTP source, (3) if the path points to an executable file, the script plugin is used, (4) otherwise static file loading is used (existing behavior).

#### Scenario: EC2 URI detected
- **WHEN** the `-i` flag value starts with `ec2://`
- **THEN** the system SHALL use the EC2 inventory plugin

#### Scenario: HTTP URL detected
- **WHEN** the `-i` flag value starts with `http://` or `https://`
- **THEN** the system SHALL use the HTTP inventory source

#### Scenario: Executable file detected
- **WHEN** the `-i` flag value points to a file with execute permission
- **THEN** the system SHALL use the script inventory plugin

#### Scenario: Static YAML file detected
- **WHEN** the `-i` flag value points to a non-executable file with a `.yaml` or `.yml` extension
- **THEN** the system SHALL use the existing static YAML inventory loader

#### Scenario: Static JSON file detected
- **WHEN** the `-i` flag value points to a non-executable file with a `.json` extension
- **THEN** the system SHALL parse the file as JSON inventory using the standard schema

### Requirement: Clear error for unrecognized inventory source
The system SHALL return a descriptive error when the `-i` flag value does not match any known source type pattern and does not point to an existing file.

#### Scenario: Non-existent file path
- **WHEN** the `-i` flag value is a path to a file that does not exist and does not match a URI scheme
- **THEN** the system SHALL return an error indicating the inventory file was not found

### Requirement: Backward compatibility with existing static inventories
The system SHALL maintain full backward compatibility with existing static YAML inventory files. Any valid inventory file that worked before this change MUST continue to work identically.

#### Scenario: Existing YAML inventory unchanged
- **WHEN** the `-i` flag points to an existing static YAML inventory file
- **THEN** the system SHALL load it using the existing YAML parser and produce identical results to the previous behavior
