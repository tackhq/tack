## ADDED Requirements

### Requirement: Module registration
The system SHALL register a `git` module in the module registry, invokable via `git:` in playbook tasks.

#### Scenario: git module is listed
- **WHEN** the module registry is listed
- **THEN** `git` SHALL be present

#### Scenario: Task dispatch
- **WHEN** a task specifies `git: { repo: ..., dest: ... }`
- **THEN** the executor SHALL dispatch it to the git module

### Requirement: Required parameters
The git module SHALL require `repo` (URL) and `dest` (absolute path). Missing either SHALL produce a validation error.

#### Scenario: Missing repo
- **WHEN** task omits `repo`
- **THEN** the task SHALL fail with a validation error

#### Scenario: Missing dest
- **WHEN** task omits `dest`
- **THEN** the task SHALL fail with a validation error

#### Scenario: Relative dest rejected
- **WHEN** `dest: "./repo"` is provided
- **THEN** the task SHALL fail with a validation error naming absolute-path requirement

### Requirement: Git binary required on target
The git module SHALL verify that `git` is available on the target before attempting operations. If missing, the task SHALL fail with a clear error instructing the user to install git.

#### Scenario: Git not installed
- **WHEN** `command -v git` fails on the target
- **THEN** the task SHALL fail with an error naming `git` as a missing prerequisite

### Requirement: Version resolution
The git module SHALL resolve `version` to a canonical SHA before any fetch. When `version` is unset, it SHALL resolve to the remote's default branch HEAD via `git ls-remote --symref <repo> HEAD`. When `version` looks like a SHA (7–40 hex chars) it SHALL be treated as a SHA directly. Otherwise it SHALL be resolved via `git ls-remote <repo> <version>`.

#### Scenario: Default version
- **WHEN** `version` is omitted
- **THEN** the module SHALL resolve HEAD of the remote default branch

#### Scenario: Branch name version
- **WHEN** `version: main`
- **THEN** the module SHALL resolve `refs/heads/main` via `git ls-remote`

#### Scenario: Tag version
- **WHEN** `version: v1.2.3`
- **THEN** the module SHALL resolve `refs/tags/v1.2.3` via `git ls-remote`

#### Scenario: SHA version
- **WHEN** `version: "abcdef1234567890abcdef1234567890abcdef12"`
- **THEN** the module SHALL treat it as a SHA without requiring ls-remote resolution

#### Scenario: Unknown ref
- **WHEN** `version` does not match any remote ref
- **THEN** the task SHALL fail with a clear error

### Requirement: Idempotency via SHA comparison
The git module SHALL compare the current HEAD SHA in `dest` against the resolved target SHA. When they match, the module SHALL skip fetch and checkout and report `changed: false`.

#### Scenario: Already at desired ref
- **WHEN** current HEAD matches resolved target SHA
- **THEN** no fetch SHALL occur and the task SHALL report `changed: false`

#### Scenario: Different SHA triggers update
- **WHEN** current HEAD differs from resolved target SHA
- **THEN** the module SHALL fetch and checkout, reporting `changed: true`

### Requirement: Clone when missing
When `dest` does not exist or is empty, the git module SHALL clone the repository into `dest` (creating parent directories as needed). The clone SHALL honor `depth`, `single_branch`, `bare`, and `version` parameters.

#### Scenario: Clone into empty dest
- **WHEN** `dest` is empty and `clone: true` (default)
- **THEN** the module SHALL run `git clone` and check out `version`

#### Scenario: Parent directories created
- **WHEN** `dest: /opt/apps/foo` and `/opt/apps/` does not exist
- **THEN** the module SHALL create `/opt/apps/` before cloning

### Requirement: `clone: false` prevents creation
When `clone: false` and `dest` is not an existing git repository, the task SHALL fail with a clear error.

#### Scenario: clone false on empty dest
- **WHEN** `clone: false` and `dest` does not contain `.git`
- **THEN** the task SHALL fail

#### Scenario: clone false on existing repo
- **WHEN** `clone: false` and `dest` is a git repo at desired ref
- **THEN** the task SHALL succeed with `changed: false`

### Requirement: `update: false` prevents fetch
When `update: false` and `dest` is an existing git repo, the git module SHALL NOT fetch or checkout and SHALL report `changed: false` regardless of current ref.

#### Scenario: Update disabled on existing repo
- **WHEN** `update: false` and repo exists at `dest`
- **THEN** no fetch/checkout SHALL occur and `changed: false` SHALL be reported

#### Scenario: Update disabled with no repo
- **WHEN** `update: false` and no repo at `dest` and `clone: true`
- **THEN** the module SHALL clone (update-false gates updates, not initial clones)

### Requirement: Dirty worktree handling
When the worktree at `dest` has uncommitted changes and a checkout would move the ref, the git module SHALL fail by default. When `force: true`, the module SHALL `git reset --hard` and `git clean -fdx` before checkout.

#### Scenario: Dirty worktree without force
- **WHEN** worktree is dirty and `force: false` and SHA would change
- **THEN** the task SHALL fail with an error naming dirty paths

#### Scenario: Dirty worktree with force
- **WHEN** worktree is dirty and `force: true` and SHA would change
- **THEN** the module SHALL reset and clean before checkout

#### Scenario: Dirty worktree, already at desired ref
- **WHEN** worktree is dirty but current SHA matches desired
- **THEN** the module SHALL NOT touch the worktree and SHALL report `changed: false`

### Requirement: Shallow clone support
When `depth > 0`, the git module SHALL perform a shallow clone/fetch using `--depth=<N>`. When `depth > 0` AND `version` is an explicit SHA, the module SHALL attempt `git fetch --depth=<N> origin <sha>`; if the server rejects SHA fetch, the module SHALL fall back to a full-depth fetch and emit a warning in the result.

#### Scenario: Shallow clone with branch
- **WHEN** `depth: 1` and `version: main`
- **THEN** the clone SHALL use `--depth=1`

#### Scenario: Shallow fetch of SHA succeeds
- **WHEN** `depth: 1`, `version` is a SHA, and server allows SHA fetches
- **THEN** the module SHALL fetch with depth 1

#### Scenario: Shallow fetch of SHA fallback
- **WHEN** `depth: 1`, `version` is a SHA, server rejects SHA fetch
- **THEN** the module SHALL fall back to a full fetch and include a warning in the result

### Requirement: Bare clone support
When `bare: true`, the git module SHALL create a bare repository at `dest` (no worktree). Worktree checks (dirty state, checkout) SHALL be skipped. `recursive` SHALL be ignored.

#### Scenario: Bare clone
- **WHEN** `bare: true` and `dest` is empty
- **THEN** the module SHALL run `git clone --bare` and produce a bare repo at `dest`

#### Scenario: Bare clone skips dirty check
- **WHEN** `bare: true` and repo exists
- **THEN** the module SHALL NOT check worktree state

### Requirement: Single-branch clone
When `single_branch: true`, the module SHALL clone only the target branch using `--single-branch --branch=<version>`.

#### Scenario: Single-branch clone
- **WHEN** `single_branch: true` and `version: main`
- **THEN** `--single-branch --branch main` flags SHALL be passed to git clone

### Requirement: Submodule support
When `recursive: true`, after a successful checkout the module SHALL run `git submodule update --init --recursive`. Authentication for submodule fetches SHALL use the same `GIT_SSH_COMMAND` as the parent repo.

#### Scenario: Recursive checkout
- **WHEN** `recursive: true` and parent has submodules
- **THEN** submodules SHALL be initialized and updated to the pinned commits

#### Scenario: Recursive on bare
- **WHEN** `recursive: true` and `bare: true`
- **THEN** submodule update SHALL be skipped

### Requirement: SSH host key acceptance
When `accept_hostkey: true`, the module SHALL set `GIT_SSH_COMMAND` to include `-o StrictHostKeyChecking=accept-new` for clone and fetch operations.

#### Scenario: First-time host with accept_hostkey
- **WHEN** `accept_hostkey: true` and the host is not in known_hosts
- **THEN** the SSH connection SHALL succeed and the host SHALL be added to known_hosts

#### Scenario: Host mismatch with accept_hostkey
- **WHEN** `accept_hostkey: true` and the host key in known_hosts differs from presented key
- **THEN** the connection SHALL fail (accept-new does not override existing keys)

### Requirement: Custom SSH key file
When `key_file` is set, the module SHALL set `GIT_SSH_COMMAND` to include `-i <key_file> -o IdentitiesOnly=yes`. `key_file` SHALL refer to a file path on the target host; the module SHALL NOT provision the key.

#### Scenario: Custom key file
- **WHEN** `key_file: /home/deploy/.ssh/id_ed25519_deploy`
- **THEN** git SHALL invoke ssh with `-i /home/deploy/.ssh/id_ed25519_deploy -o IdentitiesOnly=yes`

#### Scenario: Combined accept_hostkey and key_file
- **WHEN** both `accept_hostkey: true` and `key_file: ...` are set
- **THEN** both flags SHALL be present in GIT_SSH_COMMAND

### Requirement: Structured result
The git module SHALL return a result containing `before_sha`, `after_sha`, `remote_url`, `version_resolved`, `changed`, and `warnings`. The `before_sha` SHALL be empty when the repo did not exist before the run.

#### Scenario: Register captures SHAs
- **WHEN** task uses `register: repo_result` and a checkout occurs
- **THEN** `repo_result.before_sha` and `repo_result.after_sha` SHALL contain the old and new SHAs

#### Scenario: Initial clone before_sha
- **WHEN** `dest` did not exist and a clone occurred
- **THEN** `before_sha` SHALL be empty and `after_sha` SHALL be the resolved SHA

#### Scenario: Unchanged result
- **WHEN** repo already at desired ref
- **THEN** `before_sha` SHALL equal `after_sha` and `changed` SHALL be false

### Requirement: Check mode (--dry-run)
Under `--dry-run`, the git module SHALL perform read-only operations (`git ls-remote`, `git rev-parse HEAD`) and SHALL NOT modify the target. It SHALL report whether a change would occur.

#### Scenario: Dry-run would clone
- **WHEN** `--dry-run`, `dest` missing, `clone: true`
- **THEN** no clone SHALL occur and task SHALL report `would_change: true`

#### Scenario: Dry-run would update
- **WHEN** `--dry-run`, repo exists, SHA would change
- **THEN** no fetch/checkout SHALL occur and task SHALL report `would_change: true`

#### Scenario: Dry-run unchanged
- **WHEN** `--dry-run`, repo exists at desired SHA
- **THEN** task SHALL report `would_change: false`

### Requirement: Diff mode
Under `--diff`, the git module SHALL emit a short diff showing `before_sha → after_sha` (or "(none) → <sha>" for fresh clones).

#### Scenario: Diff on update
- **WHEN** `--diff` and SHA changes from `aaaa` to `bbbb`
- **THEN** output SHALL include `aaaa → bbbb`

#### Scenario: Diff on fresh clone
- **WHEN** `--diff` and repo did not exist
- **THEN** output SHALL include `(none) → <sha>`

### Requirement: Works through all connectors
The git module SHALL function identically through local, SSH, SSM, and Docker connectors, using only `Execute` primitives and any helpers already provided by the connector.

#### Scenario: SSH connector
- **WHEN** module runs via SSH connector
- **THEN** git commands SHALL be dispatched through the connector's Execute method

#### Scenario: Docker connector
- **WHEN** module runs via Docker connector against a container with git installed
- **THEN** the checkout SHALL occur inside the container

### Requirement: Working directory isolation
All git commands (except `clone`) SHALL pass `-C <dest>` to avoid relying on shell working directory.

#### Scenario: Rev-parse uses -C
- **WHEN** reading current HEAD
- **THEN** the command SHALL be `git -C <dest> rev-parse HEAD`
