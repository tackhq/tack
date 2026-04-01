## Why

Bolt gathers system facts (OS, architecture, hostname, EC2 metadata) but has no network facts. Users need IP addresses and interface names for templates, conditionals, and configuration — e.g., binding services to the right IP, configuring DNS, or setting up cluster peers. Ansible provides `ansible_default_ipv4`, `ansible_all_ipv4_addresses`, etc. Bolt should have equivalents.

## What Changes

- Add network discovery to the facts shell script: default IPv4 address, default interface, all IPv4 addresses, all IPv6 addresses
- Use platform-specific commands: `ip route`/`ip addr` on Linux, `route get default`/`ifconfig` on macOS
- Add `ec2_private_ip` and `ec2_public_ip` from IMDS (two additional curl calls in the existing EC2 block)
- Update `docs/variables.md` with the new facts

## Capabilities

### New Capabilities
- `network-facts`: Discover network interface and IP address information from target hosts

### Modified Capabilities

## Impact

- `pkg/facts/facts.go` — extend `factsScript` shell script and `Gather()` parser
- `docs/variables.md` — document new facts
