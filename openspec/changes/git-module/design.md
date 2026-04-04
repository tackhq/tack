## Context

Git is the universal deployment-time code delivery tool. Ansible's `ansible.builtin.git` is widely used and sets the user expectation for behavior: idempotent, ref-aware, with shallow-clone and submodule support. Bolt already has a `source/git.go` that clones repos on the **control host** as part of the source-fetching feature (for remote playbook/role sources) — this is **different**: the new module operates on **target hosts** via the connector.

Git auth on remote hosts is a thorny topic. To keep scope tight, the v1 module:
1. Uses whatever auth is available on the target (ssh-agent, `~/.ssh/id_rsa`, `key_file` when provided via `GIT_SSH_COMMAND`).
2. Does NOT provision keys, write `known_hosts`, or handle HTTPS tokens (except for `accept_hostkey` which is a targeted TOFU helper).

## Goals / Non-Goals

**Goals:**
- Idempotent: don't fetch, don't check out, don't change mtimes when the worktree already matches.
- Ref resolution: accept branches, tags, and explicit SHAs; always resolve to a canonical SHA for reporting.
- Shallow & submodule support for large repos and nested dependencies.
- Clean separation: the module calls `git` binary on the target via connector.Execute; Bolt does not link a Go git implementation.
- Useful `register:` output for downstream tasks (e.g., tagging builds with the checked-out SHA).

**Non-Goals:**
- SSH key provisioning to target (use `copy:` / `file:` / existing system setup).
- HTTPS basic-auth or token auth (deferred).
- `git submodule add` / modifications (read-only submodule updates only).
- LFS support (deferred — document as known limitation).
- Mirror-cloning / --reference (deferred).
- Windows target support.
- Pre-installed git binary is a target prerequisite — no install fallback.

## Decisions

### Decision 1: Version resolution via `git ls-remote`

To avoid a full fetch when the repo already exists and is at the desired ref, the module:
1. If `dest` exists as a git repo → read `git rev-parse HEAD` (current SHA).
2. Resolve `version`:
   - If it matches `^[0-9a-f]{7,40}$`, treat as SHA (cannot be cheaply verified without fetch → proceed to fetch path when current SHA doesn't prefix-match).
   - Otherwise invoke `git ls-remote <repo> <version>` to resolve the remote SHA.
3. If current SHA == resolved SHA → skip (report unchanged).
4. Otherwise fetch + checkout.

**Alternatives considered:**
- **Always fetch then compare**: simpler but wastes bandwidth on idempotent runs. Rejected.
- **Parse `.git/FETCH_HEAD` for last-known remote SHA**: stale data. Rejected.

### Decision 2: `version` defaults to remote default branch HEAD

If `version` is not supplied, the module resolves `HEAD` via `git ls-remote --symref <repo> HEAD` and uses the resolved branch name. This matches Ansible's behavior and avoids surprising checkouts.

### Decision 3: Dirty worktree handling

If the worktree contains uncommitted changes (`git status --porcelain` non-empty):
- `force: false` (default) → fail with clear error naming the dirty paths.
- `force: true` → `git reset --hard` + `git clean -fdx` before checkout.

Bare clones and `update: false` skip this check.

### Decision 4: `accept_hostkey` implementation

When `true`, the module sets `GIT_SSH_COMMAND="ssh -o StrictHostKeyChecking=accept-new"` for the clone/fetch invocations. This is TOFU (trust on first use). The module does **not** directly write to `known_hosts` — OpenSSH does that when using `accept-new`. Documented security trade-off.

**Alternatives considered:**
- Pre-seed `known_hosts` via ssh-keyscan: introduces its own trust question and extra commands. Rejected for v1.
- Hard `StrictHostKeyChecking=no`: unsafe, surprising. Rejected.

### Decision 5: `key_file` implementation

When set, composes `GIT_SSH_COMMAND="ssh -i <key_file> -o IdentitiesOnly=yes"`. Combined with `accept_hostkey` when both are set. The key file must already exist on the target — the module does not provision it.

### Decision 6: Idempotency-under-force

Even with `force: true`, the module first compares SHAs and skips work when already at the desired ref (force only kicks in when a change is actually being made AND the worktree is dirty). This preserves "no action = no report of change" semantics.

### Decision 7: `clone: false` semantics

When `clone: false` and `dest` is empty / not a git repo → fail with clear error. Useful for playbooks that want to *update* a pre-provisioned checkout but never create one.

### Decision 8: `update: false` semantics

When `update: false` and `dest` is a git repo → no `fetch`, no `checkout`. Just verify state and report `changed: false`. Combined with `clone: true` (default), this means "ensure it's cloned, but don't move it off current ref."

### Decision 9: Shallow clone + `version` as SHA interaction

Shallow clones can't always check out arbitrary SHAs. When `depth > 0` and `version` is a 40-char SHA:
- First try `git fetch --depth=<N> origin <sha>` (modern git supports fetching SHAs if server allows `uploadpack.allowReachableSHA1InWant`).
- Fall back to full fetch (depth dropped) with a warning surfaced to the result.

Document as a known edge case.

### Decision 10: Bare clones

When `bare: true`:
- `dest` points at a directory that will contain `HEAD`, `refs/`, `objects/` directly.
- No worktree operations (no dirty check, no checkout).
- `force` is irrelevant; `recursive` is ignored (submodules are worktree-level).
- Fetches still run when `update: true`.

### Decision 11: Submodules

When `recursive: true`, after checkout the module runs `git submodule update --init --recursive`. Submodule auth uses the same `GIT_SSH_COMMAND`. Submodule dirty state is **not** checked — only the parent worktree.

### Decision 12: Result / register output

Registered result contains:
```
changed: bool
before_sha: string  # empty if repo didn't exist before
after_sha: string
remote_url: string
version_resolved: string  # the SHA we resolved target to
warnings: [string]  # non-fatal issues (e.g. depth fallback)
```

### Decision 13: Working directory handling

All `git` invocations use `-C <dest>` to avoid `cd`. Clone uses `git clone <repo> <dest>` with sensible parent directory creation (`mkdir -p <parent>`).

### Decision 14: Sudo

The module honors connector sudo configuration. Typical use: `dest` owned by a service account → run as that user via connector auth rather than sudo. When sudo is enabled and a `become_user` is set, `git` runs via sudo.

## Risks / Trade-offs

- **[Risk]** `git ls-remote` on every run introduces a round-trip even when idempotent. → **Mitigation:** It's a lightweight refs-only listing, and the alternative (always fetching objects) is strictly worse.
- **[Risk]** `accept_hostkey: true` is TOFU — a MITM on first connect is persisted. → **Mitigation:** Default false; document the security model; recommend pre-seeded `known_hosts` for regulated environments.
- **[Risk]** Shallow-clone SHA fetch depends on server config. → **Mitigation:** Documented fallback + warning in result.
- **[Risk]** Target-side `git` version varies; some flags are recent (`--no-local`, `-c`, ...). → **Mitigation:** Stick to git 2.10+ commands (widely available on Debian 10+, RHEL 8+); document minimum.
- **[Trade-off]** No HTTPS token auth means private repos require SSH. → **Mitigation:** Document; deferred to v2.
- **[Trade-off]** No LFS. → **Mitigation:** Document; users can run `git lfs pull` via `command:` after.
- **[Risk]** Concurrent bolt runs editing the same `dest` can corrupt a checkout. → **Mitigation:** Out of scope — users should serialize. Documented.
