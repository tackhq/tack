# git

Manage git repository checkouts on targets idempotently — clone, fetch, and
checkout a pinned branch, tag, or SHA. The module compares the current HEAD
against the resolved target SHA and skips fetching or checking out when
they already match, so repeat runs produce no noise.

## Parameters

| Param            | Type   | Required | Default | Description                                                        |
|------------------|--------|----------|---------|--------------------------------------------------------------------|
| `repo`           | string | yes      | -       | Repository URL (SSH or HTTPS).                                     |
| `dest`           | string | yes      | -       | Absolute path on the target where the repo should live.            |
| `version`        | string | no       | -       | Branch, tag, or 7–40 char SHA. Defaults to the remote default HEAD.|
| `force`          | bool   | no       | false   | Reset a dirty worktree (`reset --hard` + `clean -fdx`) before checkout.|
| `depth`          | int    | no       | 0       | Shallow clone depth (0 = full clone).                              |
| `update`         | bool   | no       | true    | Fetch and checkout when the repo already exists.                   |
| `clone`          | bool   | no       | true    | Clone into `dest` when it's missing. If false, module fails when dest is empty. |
| `bare`           | bool   | no       | false   | Create a bare clone (no worktree; `force` and `recursive` are ignored).|
| `single_branch`  | bool   | no       | false   | Clone only the target branch.                                      |
| `recursive`      | bool   | no       | false   | Run `git submodule update --init --recursive` after checkout.      |
| `accept_hostkey` | bool   | no       | false   | Set `StrictHostKeyChecking=accept-new` on the SSH connection (TOFU). |
| `key_file`       | string | no       | -       | Path on the target to a private key to use for the clone/fetch.    |

## Result fields (for `register:`)

| Field              | Description                                                           |
|--------------------|-----------------------------------------------------------------------|
| `changed`          | True when clone, fetch, or checkout occurred.                         |
| `before_sha`       | HEAD SHA before the run (empty if the repo didn't exist before).      |
| `after_sha`        | HEAD SHA after the run.                                               |
| `remote_url`       | URL of the `origin` remote.                                           |
| `version_resolved` | Canonical SHA the module resolved `version` to.                       |
| `warnings`         | Non-fatal notes (e.g. shallow-fetch fallback).                        |

## Prerequisites

- `git` binary present on the target (2.10+).
- For SSH repos: the key used for auth must already exist on the target.
  This module does NOT provision keys. Use `copy:` / `file:` to place
  them, or rely on ssh-agent forwarding or existing `~/.ssh/id_*`.
- For SSH repos: the host key for the git server must be in the target's
  `known_hosts`. If it isn't, set `accept_hostkey: true` for TOFU auto-add,
  or pre-seed `known_hosts` via `ssh-keyscan` beforehand.

## Examples

### Pin to a branch (default idempotent behavior)

```yaml
- name: Deploy app code
  git:
    repo: git@github.com:acme/app.git
    dest: /opt/app
    version: main
```

### Pin to a tag

```yaml
- name: Deploy release
  git:
    repo: git@github.com:acme/app.git
    dest: /opt/app
    version: v2.4.1
```

### Pin to an exact SHA

```yaml
- name: Deploy exact commit
  git:
    repo: git@github.com:acme/app.git
    dest: /opt/app
    version: 2f9bd3a5c6e7f8a1b2c3d4e5f6a7b8c9d0e1f2a3
```

### Shallow clone for a build

```yaml
- name: Check out build source
  git:
    repo: https://github.com/acme/app.git
    dest: /build/src
    version: main
    depth: 1
    single_branch: true
```

### Bare clone (mirror)

```yaml
- name: Mirror repository
  git:
    repo: git@github.com:acme/app.git
    dest: /srv/mirrors/app.git
    bare: true
```

### Submodules

```yaml
- name: Deploy with submodules
  git:
    repo: git@github.com:acme/app.git
    dest: /opt/app
    version: main
    recursive: true
```

### Custom SSH key on target

```yaml
- name: Deploy via deploy key
  git:
    repo: git@github.com:acme/private.git
    dest: /opt/app
    key_file: /home/deploy/.ssh/id_ed25519_deploy
    accept_hostkey: true
```

### Capture SHA for downstream tasks

```yaml
- name: Check out app
  git:
    repo: git@github.com:acme/app.git
    dest: /opt/app
    version: main
  register: repo

- name: Tag the build with the checked-out SHA
  command:
    cmd: "docker build -t app:{{ repo.after_sha }} ."
```

## Idempotency

Each run issues a lightweight `git ls-remote` to resolve the target ref
to a canonical SHA, then compares it with the current `HEAD`. When they
match, no fetch or checkout occurs — no network traffic beyond the
ls-remote, no worktree changes, and `changed: false` is reported.

For a SHA `version`, the module also short-circuits when the current
`HEAD` starts with the requested SHA (prefix match), matching the git
convention of abbreviated SHAs.

## Dry-run and diff

- Under `tack run --dry-run`, the module performs only read-only
  operations (`git ls-remote`, `git rev-parse HEAD`) and reports what
  would change.
- Under `--diff`, the plan output shows `<before_sha> → <after_sha>`
  (or `(none) → <sha>` for fresh clones).

## Known limitations

- **SSH auth only** for private repos in v1. HTTPS basic-auth / token
  support is deferred to a future release.
- **No LFS support.** If you need Git LFS, run `git lfs pull` via
  `command:` after the checkout.
- **Linux/macOS targets only.** Windows targets are not supported.
- **Shallow + SHA caveat.** When `depth > 0` and `version` is an exact
  SHA, the module first attempts `git fetch --depth=N origin <sha>`; if
  the server rejects it (depends on `uploadpack.allowReachableSHA1InWant`),
  the module falls back to `git fetch --unshallow origin` and emits a
  warning in the result.
- **`accept_hostkey: false` (the safe default) will fail on first
  connection to unknown hosts.** You must pre-populate `known_hosts` on
  the target or opt into `accept_hostkey: true` (TOFU).
- **Concurrent runs on the same `dest` can corrupt the checkout.**
  Serialize access externally if your playbook might run in parallel
  against the same path.

## Security notes

- `accept_hostkey: true` is **trust-on-first-use (TOFU)**. A man-in-the-
  middle on the very first connection would be persisted into
  `known_hosts`. For regulated environments, pre-seed `known_hosts`
  using a known-good host key and keep `accept_hostkey: false`.
- `key_file` refers to a path that must already exist on the target.
  Treat it as sensitive state — provision it via a separate, trusted
  channel.
