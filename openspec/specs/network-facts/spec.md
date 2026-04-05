## ADDED Requirements

### Requirement: Default IPv4 address and interface
Tack SHALL gather the default IPv4 address and default network interface name on Linux and macOS targets.

#### Scenario: Linux target
- **WHEN** facts are gathered on a Linux host with `ip` available
- **THEN** `facts.default_ipv4` contains the primary IPv4 address and `facts.default_interface` contains the interface name (e.g., `eth0`)

#### Scenario: macOS target
- **WHEN** facts are gathered on a macOS host
- **THEN** `facts.default_ipv4` and `facts.default_interface` are populated using `route` and `ifconfig`

#### Scenario: No network available
- **WHEN** the target has no default route or the commands fail
- **THEN** the facts are absent (not set) and no error is raised

### Requirement: All IPv4 and IPv6 addresses
Tack SHALL gather all non-loopback IPv4 and IPv6 addresses as lists.

#### Scenario: Multiple interfaces
- **WHEN** the target has multiple network interfaces with IPs
- **THEN** `facts.all_ipv4` contains all non-loopback IPv4 addresses and `facts.all_ipv6` contains all non-loopback IPv6 addresses

#### Scenario: Single interface
- **WHEN** the target has one non-loopback interface
- **THEN** `facts.all_ipv4` contains that single address

### Requirement: EC2 private and public IP from IMDS
Tack SHALL gather EC2 private and public IPv4 addresses from IMDS when running on EC2.

#### Scenario: EC2 instance with public IP
- **WHEN** facts are gathered on an EC2 instance with a public IP
- **THEN** `facts.ec2_private_ip` and `facts.ec2_public_ip` are populated

#### Scenario: EC2 instance without public IP
- **WHEN** the instance has no public IP
- **THEN** `facts.ec2_private_ip` is populated and `facts.ec2_public_ip` is absent

#### Scenario: Non-EC2 host
- **WHEN** IMDS is unreachable
- **THEN** neither `ec2_private_ip` nor `ec2_public_ip` is set
