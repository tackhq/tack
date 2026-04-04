## ADDED Requirements

### Requirement: EC2 instance discovery
The EC2 plugin SHALL call `DescribeInstances` with configured tag filters and return discovered instances as inventory hosts.

#### Scenario: Discover instances by tags
- **WHEN** config specifies `filters: {tag:env: production}` and 3 matching instances exist
- **THEN** the plugin SHALL return an `*Inventory` with 3 host entries

#### Scenario: No instances match
- **WHEN** config specifies filters that match zero instances
- **THEN** the plugin SHALL return an empty `*Inventory` (not an error)

#### Scenario: AWS API error
- **WHEN** the `DescribeInstances` call fails (permissions, throttling)
- **THEN** the plugin SHALL return an error wrapping the AWS SDK error

### Requirement: EC2 plugin configuration
The plugin SHALL be configured via YAML with `plugin: ec2` and the following fields:
- `regions` (required): List of AWS regions to query
- `filters` (optional): Map of EC2 filter name to value (e.g., `tag:env: production`)
- `group_by` (optional): List of tag keys to create groups from (e.g., `[tag:role, tag:env]`)
- `host_key` (optional): Which value to use as host identifier — `private_ip` (default), `public_ip`, or `instance_id`

#### Scenario: Multi-region discovery
- **WHEN** config specifies `regions: [us-east-1, us-west-2]`
- **THEN** the plugin SHALL query both regions and merge results into a single inventory

#### Scenario: Instance ID as host key
- **WHEN** config specifies `host_key: instance_id`
- **THEN** each host entry SHALL be keyed by its EC2 instance ID (e.g., `i-0abc1234`)

#### Scenario: Private IP as host key (default)
- **WHEN** `host_key` is not specified
- **THEN** each host entry SHALL be keyed by its private IP address

#### Scenario: Missing regions
- **WHEN** config does not include a `regions` field
- **THEN** the plugin SHALL return a validation error

### Requirement: Auto-grouping by tags
The plugin SHALL create inventory groups based on EC2 instance tags when `group_by` is configured. Group names SHALL follow the pattern `tag_{key}_{value}` with non-alphanumeric characters replaced by underscores.

#### Scenario: Group by role tag
- **WHEN** config specifies `group_by: [tag:role]` and instances have `role=worker` and `role=api`
- **THEN** the inventory SHALL contain groups `tag_role_worker` and `tag_role_api` with the corresponding instances

#### Scenario: Instance in multiple groups
- **WHEN** config specifies `group_by: [tag:role, tag:env]` and an instance has both tags
- **THEN** the instance SHALL appear in both the role-based and env-based groups

#### Scenario: Instance missing a group_by tag
- **WHEN** an instance does not have a tag listed in `group_by`
- **THEN** the instance SHALL be omitted from that tag's groups but still appear in the inventory hosts

### Requirement: Instance tags as host vars
The plugin SHALL add all EC2 tags as host variables on each discovered instance. Tag keys SHALL be lowercased with hyphens replaced by underscores.

#### Scenario: Tags mapped to vars
- **WHEN** an instance has tags `Name=web-01`, `env=production`
- **THEN** the host entry SHALL have vars `{name: "web-01", env: "production"}`

### Requirement: Connection defaults for EC2
The plugin SHALL set connection defaults on discovered hosts based on the `host_key` value:
- `instance_id` → connection: `ssm`
- `private_ip` or `public_ip` → connection: `ssh`

#### Scenario: SSM connection for instance_id host key
- **WHEN** config specifies `host_key: instance_id`
- **THEN** discovered hosts SHALL have their group connection set to `ssm`

#### Scenario: SSH connection for IP host key
- **WHEN** config specifies `host_key: private_ip`
- **THEN** discovered hosts SHALL have their group connection set to `ssh`
