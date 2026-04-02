## ADDED Requirements

### Requirement: Discover EC2 instances by tags and filters
The system SHALL discover EC2 instances using the AWS EC2 DescribeInstances API when the `-i` flag value starts with `ec2://`. The region SHALL be specified as the URI path segment. Query parameters prefixed with `tag:` SHALL become tag-based filters, and other query parameters SHALL become standard EC2 filters.

#### Scenario: Discover instances by tag
- **WHEN** the `-i` flag is `ec2://us-east-1?tag:Environment=production`
- **THEN** the system SHALL call DescribeInstances in us-east-1 with a tag filter for Environment=production and return matching running instances as inventory hosts

#### Scenario: Discover instances by multiple filters
- **WHEN** the `-i` flag is `ec2://us-west-2?tag:Role=web&vpc-id=vpc-abc123`
- **THEN** the system SHALL call DescribeInstances with both the tag filter and the vpc-id filter, returning only instances matching all filters

#### Scenario: No instances match filters
- **WHEN** the EC2 API returns zero instances matching the specified filters
- **THEN** the system SHALL return an empty but valid Inventory (no hosts, no groups)

### Requirement: Auto-populate SSH config from instance metadata
For each discovered EC2 instance, the system SHALL populate the host's SSH configuration from instance metadata. The SSH host SHALL be set to the public IP address if available, otherwise the private IP address. The SSH user SHALL default to a well-known user based on the AMI platform (e.g., `ec2-user` for Amazon Linux, `ubuntu` for Ubuntu). The SSH key name SHALL be set from the instance's KeyName attribute.

#### Scenario: Instance with public IP
- **WHEN** an EC2 instance has a public IP address assigned
- **THEN** the system SHALL use the public IP as the SSH host address

#### Scenario: Instance without public IP
- **WHEN** an EC2 instance has no public IP address
- **THEN** the system SHALL use the private IP address as the SSH host address

#### Scenario: Instance with key pair
- **WHEN** an EC2 instance has a KeyName attribute set
- **THEN** the system SHALL set the SSH key path to `~/.ssh/<KeyName>.pem`

### Requirement: Populate host vars from instance tags
The system SHALL populate each host's `vars` map with all EC2 instance tags as key-value pairs. The instance ID SHALL be available as the var `instance_id`, and the instance type as `instance_type`.

#### Scenario: Instance with tags
- **WHEN** an EC2 instance has tags `Environment=production` and `Role=web`
- **THEN** the host vars SHALL contain `Environment: production`, `Role: web`, `instance_id: <id>`, and `instance_type: <type>`

### Requirement: Group instances by tag values
The system SHALL automatically create inventory groups based on tag values. For each unique tag key-value pair across discovered instances, a group SHALL be created named `tag_<Key>_<Value>` containing all instances with that tag.

#### Scenario: Instances grouped by Environment tag
- **WHEN** three instances are discovered, two with `Environment=production` and one with `Environment=staging`
- **THEN** the inventory SHALL contain groups `tag_Environment_production` (2 hosts) and `tag_Environment_staging` (1 host)

### Requirement: Only include running instances
The system SHALL only include EC2 instances in the `running` state (instance-state-name = running). Terminated, stopped, or pending instances SHALL be excluded.

#### Scenario: Mix of instance states
- **WHEN** EC2 returns instances in running, stopped, and terminated states
- **THEN** only running instances SHALL appear in the inventory

### Requirement: Use standard AWS credential chain
The system SHALL use the standard AWS SDK v2 credential chain for authentication, supporting environment variables, shared credentials file, and IAM roles. No bolt-specific AWS credential configuration SHALL be required.

#### Scenario: Credentials from environment
- **WHEN** AWS credentials are set via `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` environment variables
- **THEN** the EC2 plugin SHALL use those credentials to call DescribeInstances
