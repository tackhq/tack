# Connectors

Connectors define how Bolt connects to and executes commands on target systems.

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

Supports sudo via `sudo: true` at play or task level. Password can be provided via `--sudo-password` flag, `BOLT_SUDO_PASSWORD` env var, or interactive prompt.

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
bolt run playbook.yaml -c docker://my-container
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
bolt run playbook.yaml -c ssh://deploy@web1:2222
bolt run playbook.yaml -c ssh://deploy@web1 -c ssh://deploy@web2

# Separate flags
bolt run playbook.yaml --hosts web1,web2 --ssh-user deploy --ssh-key ~/.ssh/deploy_key

# SSH config aliases work directly
bolt run playbook.yaml --hosts myserver

# Connection type is auto-detected from SSH flags or remote hosts
bolt run playbook.yaml --hosts web1 --ssh-user deploy
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `BOLT_CONNECTION` | Connection type |
| `BOLT_HOSTS` | Comma-separated host list |
| `BOLT_SSH_USER` | SSH username |
| `BOLT_SSH_PORT` | SSH port |
| `BOLT_SSH_KEY` | Path to SSH private key |
| `BOLT_SSH_PASSWORD` | SSH password |
| `BOLT_SSH_INSECURE` | Skip host key verification (`1`, `true`, or `yes`) |

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
bolt run patch.yaml --ssm-tags env=production,role=app-server --ssm-region us-east-1

# Direct instance IDs
bolt run patch.yaml --ssm-instances i-0abc123,i-0def456 --ssm-region us-east-1 --ssm-bucket my-bucket
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `BOLT_SSM_INSTANCES` | Comma-separated instance IDs |
| `BOLT_SSM_TAGS` | Comma-separated key=value tags |
| `BOLT_SSM_REGION` | AWS region |
| `BOLT_SSM_BUCKET` | S3 bucket for file transfer |

AWS credentials use the standard SDK credential chain (env vars, shared config, IAM roles).

## Dynamic Inventory

In addition to static YAML inventory files, Bolt supports dynamic inventory sources via a plugin architecture. Pass any source with `-i`:

**Executable scripts** — auto-detected by file permissions, run with `--list`:
```bash
bolt run deploy.yaml -i ./my-inventory-script.sh
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
bolt run deploy.yaml -i ec2.yml -i overrides.yml
```

**Inspect resolved inventory** for debugging:
```bash
bolt inventory --list -i ec2.yml
bolt inventory --host web1 -i hosts.yml
```

Use `--inventory-timeout` to control plugin execution timeout (default: 30s).

See [`examples/dynamic-inventory/`](../examples/dynamic-inventory/) for complete samples.

## Auto-Detection

When no `connection:` is specified, Bolt infers the type from flags:

- SSH flags (`--ssh-user`, `--ssh-key`, etc.) or remote `--hosts` values imply `ssh`
- SSM flags (`--ssm-instances`, `--ssm-tags`) imply `ssm`
- Otherwise defaults to `local`

## Parallel Execution

Use `--forks N` (or `BOLT_FORKS` env var) to execute against multiple hosts concurrently. Output is buffered per-host and flushed in host order after completion. Defaults to 1 (serial).

```bash
bolt run deploy.yaml --hosts web1,web2,web3 --forks 3
```
