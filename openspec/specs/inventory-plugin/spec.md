## ADDED Requirements

### Requirement: Plugin interface
The system SHALL define an `InventoryPlugin` interface with `Name() string` and `Load(ctx context.Context, config map[string]any) (*Inventory, error)` methods. All inventory plugins MUST implement this interface.

#### Scenario: Plugin returns valid inventory
- **WHEN** a plugin's `Load` method is called with valid configuration
- **THEN** it SHALL return a populated `*Inventory` with hosts and/or groups, or an error

#### Scenario: Plugin returns error
- **WHEN** a plugin's `Load` method encounters a failure (network, auth, parse error)
- **THEN** it SHALL return a nil inventory and a descriptive error wrapping the cause

### Requirement: Plugin registry
The system SHALL maintain a registry of built-in plugins keyed by name. Plugins MUST be registered at init time. The registry SHALL support lookup by name string.

#### Scenario: Lookup registered plugin
- **WHEN** a plugin config file specifies `plugin: http`
- **THEN** the registry SHALL return the HTTP plugin implementation

#### Scenario: Lookup unknown plugin
- **WHEN** a plugin config file specifies `plugin: unknown`
- **THEN** the registry SHALL return an error indicating the plugin name is not recognized

### Requirement: Auto-detection routing
`inventory.Load()` SHALL route to the correct handler based on the input path:
1. If the path points to an executable file → script plugin
2. If the YAML content contains a `plugin:` key → named plugin from registry
3. Otherwise → static YAML parse (existing behavior, unchanged)

#### Scenario: Executable file detected
- **WHEN** `-i ./my-script` is provided and `./my-script` has the executable bit set
- **THEN** the system SHALL invoke the script plugin, not attempt YAML parsing

#### Scenario: Plugin key in YAML
- **WHEN** `-i ./inventory.yml` is provided and the file contains `plugin: ec2`
- **THEN** the system SHALL dispatch to the EC2 plugin with the file contents as config

#### Scenario: Static YAML fallback
- **WHEN** `-i ./hosts.yml` is provided, the file is not executable, and has no `plugin:` key
- **THEN** the system SHALL parse it as a static inventory (current behavior preserved)

### Requirement: Plugin timeout
All plugin `Load` calls SHALL be wrapped with a context timeout. The default timeout SHALL be 30 seconds. The timeout MUST be configurable via `--inventory-timeout` CLI flag or `timeout:` key in plugin config.

#### Scenario: Plugin exceeds timeout
- **WHEN** a plugin's Load takes longer than the configured timeout
- **THEN** the context SHALL be cancelled and Load SHALL return a timeout error

#### Scenario: Custom timeout in config
- **WHEN** a plugin config file contains `timeout: 10`
- **THEN** the plugin Load SHALL use a 10-second timeout instead of the 30-second default
