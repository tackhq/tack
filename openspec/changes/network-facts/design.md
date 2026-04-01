## Context

Bolt's fact-gathering runs a single shell script on the target and parses `BOLT_FACT key=value` lines. Network facts need platform-specific commands but fit the same pattern. EC2 IPs are available from the existing IMDS flow.

## Goals / Non-Goals

**Goals:**
- `facts.default_ipv4`, `facts.default_interface` — the primary IP and interface
- `facts.all_ipv4`, `facts.all_ipv6` — all addresses as lists
- `facts.ec2_private_ip`, `facts.ec2_public_ip` — from IMDS
- Portable across Linux and macOS

**Non-Goals:**
- Per-interface detail maps (MAC, MTU, netmask, broadcast)
- IPv6 default route/interface
- Network facts on Windows or other platforms

## Decisions

### 1. Platform-specific shell branches

Linux: `ip route get 1` for default interface/IP, `ip -4 addr` for all IPs.
macOS: `route -n get default` for default interface, `ifconfig <iface>` for default IP, `ifconfig` for all IPs.

Both paths already exist in the script (Darwin vs Linux branches).

### 2. List facts use comma-delimited BOLT_FACT lines

`all_ipv4` and `all_ipv6` are lists. Emit as `BOLT_FACT all_ipv4=10.0.0.1,172.17.0.1` and split on comma in Go. This avoids block parsing (like os_release) for a simple case.

### 3. EC2 IPs are two more curl calls in the existing IMDS block

No new infrastructure — just `local-ipv4` and `public-ipv4` endpoints.

## Risks / Trade-offs

- **[Risk] `ip` command not available on minimal containers** — Fallback: skip network facts gracefully (exec 2>/dev/null already suppresses errors, facts will just be absent).
- **[Risk] Multiple default routes** — `ip route get 1` picks the kernel's preferred route, which is the right behavior.
