## ADDED Requirements

### Requirement: Package installation
The `yum` module SHALL install one or more packages when `state` is `present` (or omitted, as `present` is the default). The module SHALL report `Changed: false` if all specified packages are already installed.

#### Scenario: Install a single package
- **WHEN** `name: nginx` and `state: present`
- **THEN** the module SHALL install nginx via yum/dnf and report `Changed: true`

#### Scenario: Package already installed
- **WHEN** `name: nginx` and `state: present` and nginx is already installed
- **THEN** the module SHALL report `Changed: false`

#### Scenario: Install multiple packages
- **WHEN** `name: [nginx, curl, wget]` and `state: present`
- **THEN** the module SHALL install all packages not yet installed and report `Changed: true` only if at least one was newly installed

### Requirement: Package removal
The `yum` module SHALL remove one or more packages when `state` is `absent`. The module SHALL report `Changed: false` if none of the specified packages are installed.

#### Scenario: Remove an installed package
- **WHEN** `name: nginx` and `state: absent` and nginx is installed
- **THEN** the module SHALL remove nginx and report `Changed: true`

#### Scenario: Remove a package that is not installed
- **WHEN** `name: nginx` and `state: absent` and nginx is not installed
- **THEN** the module SHALL report `Changed: false`

### Requirement: Package upgrade to latest
The `yum` module SHALL upgrade packages to the latest available version when `state` is `latest`. If a package is not installed, it SHALL be installed. The module SHALL report `Changed: false` if all packages are already at the latest version.

#### Scenario: Upgrade an outdated package
- **WHEN** `name: nginx` and `state: latest` and an update is available
- **THEN** the module SHALL upgrade nginx and report `Changed: true`

#### Scenario: Package already at latest version
- **WHEN** `name: nginx` and `state: latest` and nginx is already at the latest version
- **THEN** the module SHALL report `Changed: false`

#### Scenario: Install missing package with state latest
- **WHEN** `name: nginx` and `state: latest` and nginx is not installed
- **THEN** the module SHALL install the latest version of nginx and report `Changed: true`

### Requirement: Cache update
The `yum` module SHALL refresh the package metadata cache when `update_cache: true`. This SHALL run before any package operations in the same task.

#### Scenario: Update cache before install
- **WHEN** `update_cache: true` and `name: nginx` and `state: present`
- **THEN** the module SHALL run cache update first, then install nginx

#### Scenario: Update cache only
- **WHEN** `update_cache: true` and no `name` is specified
- **THEN** the module SHALL update the cache and report `Changed: true`

### Requirement: Upgrade all packages
The `yum` module SHALL upgrade all installed packages when `upgrade: yes`.

#### Scenario: Upgrade all packages
- **WHEN** `upgrade: yes`
- **THEN** the module SHALL run `yum update -y` (or `dnf upgrade -y`) and report `Changed: true`

### Requirement: Autoremove unused dependencies
The `yum` module SHALL remove unused dependency packages when `autoremove: true`.

#### Scenario: Autoremove after package removal
- **WHEN** `autoremove: true`
- **THEN** the module SHALL run `yum autoremove -y` (or `dnf autoremove -y`)

### Requirement: Auto-detect yum or dnf
The module SHALL auto-detect whether the target system has `dnf` or `yum` available. It SHALL prefer `dnf` when both are present. The module SHALL return an error if neither is found.

#### Scenario: System has dnf
- **WHEN** the target system has `dnf` in PATH
- **THEN** the module SHALL use `dnf` for all package operations

#### Scenario: System has only yum
- **WHEN** the target system has `yum` but not `dnf`
- **THEN** the module SHALL use `yum` for all package operations

#### Scenario: System has neither
- **WHEN** the target system has neither `yum` nor `dnf`
- **THEN** the module SHALL return an error

### Requirement: Dry-run support
The `yum` module SHALL implement the `Checker` interface to support check mode (dry-run). Check mode SHALL query current state without modifying the system.

#### Scenario: Check mode reports would-change
- **WHEN** check mode is active and `name: nginx` and `state: present` and nginx is not installed
- **THEN** the module SHALL report `WouldChange: true` without installing anything

#### Scenario: Check mode reports no change
- **WHEN** check mode is active and `name: nginx` and `state: present` and nginx is already installed
- **THEN** the module SHALL report `WouldChange: false`

### Requirement: Idempotent state queries
The module SHALL use `rpm -q <package>` to determine whether a package is currently installed. It SHALL NOT rely on yum/dnf output parsing for state determination.

#### Scenario: Query installed package
- **WHEN** querying whether nginx is installed and `rpm -q nginx` exits with code 0
- **THEN** the module SHALL consider nginx as installed

#### Scenario: Query missing package
- **WHEN** querying whether nginx is installed and `rpm -q nginx` exits with non-zero code
- **THEN** the module SHALL consider nginx as not installed

### Requirement: Non-interactive execution
All yum/dnf commands SHALL include the `-y` flag to prevent interactive prompts. The module SHALL NOT require user interaction during execution.

#### Scenario: Install without prompts
- **WHEN** installing a package
- **THEN** the module SHALL pass `-y` to the yum/dnf command
