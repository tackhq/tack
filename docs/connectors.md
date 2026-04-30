# Connectors

Connectors define how Tack connects to and executes commands on target systems.

## Available Connectors

| Connector | Syntax | Description |
|-----------|--------|-------------|
| **Local** | `connection: local` | Execute on the local machine |
| **Docker** | `connection: docker` | Execute inside a Docker container |
| **SSH** | `connection: ssh` or `-c ssh://user@host:port` | Connect via SSH |
| **SSM** | `connection: ssm` | Connect via AWS Systems Manager |

## Local Connector

Execute commands on the local machine. This is the default when no connection is specified.

```yaml
name: Local Setup
hosts: localhost
connection: local

tasks:
  - name: Install packages
    brew:
      name: [git, go]
      state: present
```

Supports sudo via `sudo: true` at play or task level. Password can be provided via `--sudo-password` flag, `TACK_SUDO_PASSWORD` env var, or interactive prompt.

## Docker Connector

Execute commands inside Docker containers using `docker exec`. File transfer uses `docker cp`.

```yaml
name: Configure Container
hosts: my-container
connection: docker

tasks:
  - name: Install curl
    command:
      cmd: apt-get update && apt-get install -y curl
```

The `hosts` value is the container name or ID. Sudo runs commands as the specified user inside the container.

**CLI shorthand:**

```bash
tack run playbook.yaml -c docker://my-container
```

## SSH Connector

Connect to remote hosts via SSH. Supports key-based and password authentication, and reads `~/.ssh/config` and `~/.ssh/known_hosts` automatically.

### Configuration Sources

SSH settings can come from multiple sources. Priority (highest first):

1. CLI flags (`--ssh-user`, `--ssh-port`, `--ssh-key`, `--ssh-password`, `--ssh-insecure`)
2. Playbook `ssh:` block
3. Per-host inventory `ssh:` settings
4. Group inventory `ssh:` settings
5. `~/.ssh/config`
6. Defaults

### Playbook Configuration

```yaml
name: Configure Web Server
hosts: [web1, web2]
connection: ssh

ssh:
  user: deploy
  key: ~/.ssh/deploy_key
  port: 22

tasks:
  - name: Install nginx
    apt:
      name: nginx
      state: present
```

### CLI Usage

```bash
# URI-style connection strings
tack run playbook.yaml -c ssh://deploy@web1:2222
tack run playbook.yaml -c ssh://deploy@web1 -c ssh://deploy@web2

# Separate flags
tack run playbook.yaml --hosts web1,web2 --ssh-user deploy --ssh-key ~/.ssh/deploy_key

# SSH config aliases work directly
tack run playbook.yaml --hosts myserver

# Connection type is auto-detected from SSH flags or remote hosts
tack run playbook.yaml --hosts web1 --ssh-user deploy
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `TACK_CONNECTION` | Connection type |
| `TACK_HOSTS` | Comma-separated host list |
| `TACK_SSH_USER` | SSH username |
| `TACK_SSH_PORT` | SSH port |
| `TACK_SSH_KEY` | Path to SSH private key |
| `TACK_SSH_PASSWORD` | SSH password |
| `TACK_SSH_INSECURE` | Skip host key verification (`1`, `true`, or `yes`) |

## SSM Connector

Connect to AWS EC2 instances via Systems Manager. No SSH keys required - uses IAM-based authentication. Works with private instances that have no public IP.

### Tag-Based Discovery

SSM can discover instances by EC2 tags at runtime:

```yaml
name: Patch App Servers
connection: ssm

ssm:
  region: us-east-1
  bucket: my-ssm-transfer-bucket
  tags:
    env: production
    role: app-server

tasks:
  - name: Install security updates
    apt:
      name: "*"
      state: latest
```

The `bucket` field is required for file upload/download operations (copy, template modules).

### Direct Instance IDs

```yaml
name: Configure Instances
connection: ssm
hosts: [i-0abc123, i-0def456]

ssm:
  region: us-east-1
  bucket: my-transfer-bucket
```

### CLI Usage

```bash
# Tags on CLI (SSM connection auto-detected)
tack run patch.yaml --ssm-tags env=production,role=app-server --ssm-region us-east-1

# Direct instance IDs
tack run patch.yaml --ssm-instances i-0abc123,i-0def456 --ssm-region us-east-1 --ssm-bucket my-bucket
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `TACK_SSM_INSTANCES` | Comma-separated instance IDs |
| `TACK_SSM_TAGS` | Comma-separated key=value tags |
| `TACK_SSM_REGION` | AWS region |
| `TACK_SSM_BUCKET` | S3 bucket for file transfer |

AWS credentials use the standard SDK credential chain (env vars, shared config, IAM roles).

## Dynamic Inventory

In addition to static YAML inventory files, Tack supports dynamic inventory sources via a plugin architecture. Pass any source with `-i`:

**Executable scripts** — auto-detected by file permissions, run with `--list`:
```bash
tack run deploy.yaml -i ./my-inventory-script.sh
```

**Plugin configs** — YAML files with a `plugin:` key:
```yaml
# HTTP: fetch from REST API
plugin: http
url: https://cmdb.example.com/api/inventory
headers:
  Authorization: "Bearer {{ env.CMDB_TOKEN }}"

# EC2: discover AWS instances by tags
plugin: ec2
regions: [us-east-1]
filters:
  tag:env: production
group_by: [tag:role]
host_key: private_ip
```

**Multiple sources** merge in order (later wins on conflicts):
```bash
tack run deploy.yaml -i ec2.yml -i overrides.yml
```

**Inspect resolved inventory** for debugging:
```bash
tack inventory --list -i ec2.yml
tack inventory --host web1 -i hosts.yml
```

Use `--inventory-timeout` to control plugin execution timeout (default: 30s).

See [`examples/dynamic-inventory/`](../examples/dynamic-inventory/) for complete samples.

## Auto-Detection

When no `connection:` is specified, Tack infers the type from flags:

- SSH flags (`--ssh-user`, `--ssh-key`, etc.) or remote `--hosts` values imply `ssh`
- SSM flags (`--ssm-instances`, `--ssm-tags`) imply `ssm`
- Otherwise defaults to `local`

## Parallel Execution

Use `--forks N` (or `TACK_FORKS` env var) to execute against multiple hosts concurrently. Output is buffered per-host and flushed in host order after completion. Defaults to 1 (serial).

```bash
tack run deploy.yaml --hosts web1,web2,web3 --forks 3
```

### Parallel Fact Gathering

Fact gathering runs concurrently across all target hosts regardless of `--forks`. For multi-host plays the executor opens connectors and runs `Gathering Facts` for every host in parallel before any plan/apply work begins, then reuses the open connector for the apply phase.

This is most visible on slow connectors like SSM, where each round-trip costs several seconds. A four-host SSM play that previously waited `4 × t` for serial fact gather now waits roughly `t` (bounded by the slowest host).

The pre-pass is skipped for single-host plays, `connection: local`, and plays with `gather_facts: false`. Concurrency is internally capped at 20 to avoid overwhelming AWS API limits on very large fleets.

### Multi-host Plan & Approval

Plays targeting more than one host render a single consolidated plan with per-line host attribution and prompt for approval **once globally**, not per-host. Output looks like:

```
PLAN
web1: + apt: install nginx
web2: ~ command: rotate cert

Plan: 1 to change, 1 to run across 2 hosts.

Do you want to apply these changes? (yes/no):
```

Hostnames are column-aligned (capped at 30 characters; longer names truncate with `…`). Hosts whose plan contains only no-op tasks contribute zero body lines and are counted in the footer as `(N unchanged)`.

The approval prompt always runs on the main thread, even with `--forks > 1`. SIGINT during the prompt aborts the play with zero hosts applied.

Single-host plays continue to use the existing per-host plan format (no host prefix), so common-case output is unchanged.
