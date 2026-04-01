## 1. Shell script — network discovery

- [x] 1.1 Add Linux network facts to factsScript: default_ipv4, default_interface via `ip route get 1`, all_ipv4 via `ip -4 addr`, all_ipv6 via `ip -6 addr`
- [x] 1.2 Add macOS network facts to factsScript: default_interface via `route -n get default`, default_ipv4 via `ifconfig`, all_ipv4/all_ipv6 via `ifconfig`
- [x] 1.3 Add ec2_private_ip and ec2_public_ip curl calls in the existing IMDS block

## 2. Go parser — handle new facts

- [x] 2.1 Parse default_ipv4, default_interface, ec2_private_ip, ec2_public_ip as simple string facts
- [x] 2.2 Parse all_ipv4 and all_ipv6 comma-delimited values into []string slices

## 3. Tests

- [x] 3.1 Add unit tests for parsing the new BOLT_FACT lines (mock stdout with network fact lines)
- [x] 3.2 Test comma-delimited list parsing for all_ipv4/all_ipv6 (single IP, multiple IPs, empty)

## 4. Documentation

- [x] 4.1 Add network facts section to docs/variables.md
