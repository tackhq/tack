## Why

Deployment playbooks commonly need to check out a git repository at a specific ref on target hosts — to pull application code, configuration, templates, or scripts. Bolt currently has no first-class way to do this; users fall back to `command:` tasks that shell out to `git`, losing idempotency and producing noisy diffs on every run. A proper `git` module compares current HEAD to desired ref and only fetches/checks out when actually needed.

## What Changes

- Add a `git` module that manages repository checkouts on targets idempotently.
- Params: `repo` (required, SSH or HTTPS URL), `dest` (required, absolute path on target), `version` (branch/tag/SHA, defaults to remote default branch HEAD), `force` (bool, reset dirty worktree, default false), `depth` (int, shallow clone depth; 0 = full clone), `accept_hostkey` (bool, auto-add host to known_hosts on first connect, default false), `update` (bool, skip fetch when repo already exists, default true), `clone` (bool, fail if dest doesn't contain a clone, default true), `bare` (bool, create bare clone), `single_branch` (bool, clone only the target branch), `recursive` (bool, fetch submodules), `key_file` (optional path to SSH private key on target).
- Idempotency: resolve `version` to a commit SHA via `git ls-remote` (or local ref resolution after fetch), compare against current `HEAD`, skip fetch/checkout when already at the desired SHA.
- Requires `git` binary on the target host; returns a clear error if not present.
- Supports check mode (`--dry-run`) and `--diff` (shows `before_sha → after_sha`).
- SSH-based auth only in v1: assumes keys are already present on the target (explicit non-goal: key provisioning). HTTPS authentication with tokens is deferred to a future change.
- Returns a structured result: `{before_sha, after_sha, remote_url, version_resolved}` available via `register:`.

## Capabilities

### New Capabilities
- `git-module`: Idempotent management of git repository checkouts on targets — clone, update, and checkout-at-ref — with support for shallow clones, submodules, bare repos, dirty-worktree handling, and SSH-based authentication via existing target-side keys.

### Modified Capabilities

None.

## Impact

- New package: `internal/module/git/`
- Module registry: one new module auto-registered via `init()`
- No changes to existing code, APIs, or dependencies
- No new external dependencies — requires `git` binary on target (documented prerequisite)
- Documentation: add `git` to `README.md`, `docs/modules/`, `llms.txt`, and an example playbook
- Shorthand expansion in `internal/playbook/` will need `git` recognized (if that registry is explicit)
