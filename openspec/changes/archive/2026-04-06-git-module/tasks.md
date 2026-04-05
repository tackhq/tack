## 1. Package Scaffold

- [x] 1.1 Create `internal/module/git/` package with `git.go` module struct, `Name() string`, and `init()` registration
- [x] 1.2 Implement `Describer` with all params documented in ParamDoc
- [x] 1.3 Implement `Checker` stub for dry-run support

## 2. Parameter Parsing & Validation

- [x] 2.1 Parse all params via `module/params.go` helpers
- [x] 2.2 Validate `repo` is non-empty; `dest` is non-empty and absolute path
- [x] 2.3 Validate mutual-exclusion / defaults for bools: `update` default true, `clone` default true, `force`/`bare`/`single_branch`/`recursive`/`accept_hostkey` default false
- [x] 2.4 Validate `depth >= 0`
- [x] 2.5 Detect whether `version` is SHA-like (`^[0-9a-f]{7,40}$`) for downstream logic
- [x] 2.6 Validate `version`: if provided, must be non-empty trimmed string; SHA detection is informational (not restrictive)

## 3. Git Binary Precheck

- [x] 3.1 Implement `ensureGit(ctx, conn)` — runs `command -v git` (or `which git`); returns clear error when missing
- [x] 3.2 Cache the result within a single Run (no redundant checks)

## 4. Version Resolution

- [x] 4.1 Implement `resolveVersion(ctx, conn, repo, version, sshCmd)` — handles three cases: unset (resolve HEAD via `git ls-remote --symref`), SHA-like (pass through), else `git ls-remote <repo> <refs>`
- [x] 4.2 Construct `GIT_SSH_COMMAND` env var from `accept_hostkey` + `key_file` and apply to ls-remote
- [x] 4.3 Return `{resolvedSHA, resolvedRef}` or error naming unresolved ref

## 5. Repo State Inspection

- [x] 5.1 Implement `isGitRepo(ctx, conn, dest)` — check for `.git` (worktree) or `HEAD` file (bare)
- [x] 5.2 Implement `currentSHA(ctx, conn, dest)` — `git -C <dest> rev-parse HEAD`
- [x] 5.3 Implement `isDirty(ctx, conn, dest)` — `git -C <dest> status --porcelain` non-empty (skip for bare)
- [x] 5.4 Implement `currentRemoteURL(ctx, conn, dest)` — `git -C <dest> remote get-url origin`

## 6. Clone Path

- [x] 6.1 Implement `clone(ctx, conn, params, sshCmd)` — creates parent dir with `mkdir -p`, then `git clone` with appropriate flags (`--bare`, `--depth=N`, `--single-branch --branch=<ref>`, repo, dest)
- [x] 6.2 Run submodule update after clone when `recursive: true` and not bare
- [x] 6.3 For fresh clones where `version` is not a branch/tag but a SHA, checkout the SHA after clone

## 7. Update Path

- [x] 7.1 Implement `fetch(ctx, conn, dest, params, sshCmd)` — `git -C <dest> fetch origin [--depth=N]` with SHA-fetch fallback logic and warning collection. When fetching a bare SHA on an existing shallow clone fails, fall back to `git fetch --unshallow origin` and append a warning to `Result.Data.warnings` explaining the depth was extended.
- [x] 7.2 Implement `checkout(ctx, conn, dest, sha)` — `git -C <dest> checkout --detach <sha>` (use --detach to avoid branch-HEAD mutation surprises)
- [x] 7.3 Run submodule update after checkout when `recursive: true` and not bare
- [x] 7.4 Handle `force: true` via `git -C <dest> reset --hard && git -C <dest> clean -fdx` before checkout (skip for bare)

## 8. Run / Dispatch Orchestration

- [x] 8.1 Run wiring: validate → ensureGit → if !exists && !clone: fail → if exists: currentSHA → resolveVersion → if match: return unchanged → if !update && exists: return unchanged → dirty-check → clone-or-update → post-verify SHA
- [x] 8.2 Populate Result.Data with `{before_sha, after_sha, remote_url, version_resolved, warnings: [...]}`
- [x] 8.3 Return `Changed(msg)` / `Unchanged(msg)` with message describing action

## 9. Check Mode

- [x] 9.1 Implement `Check()` that performs read-only inspection + resolveVersion and returns `WouldChange`/`NoChange` without clone/fetch/checkout
- [x] 9.2 Populate OldChecksum/NewChecksum fields with before_sha/after_sha (reuse existing CheckResult fields)

## 10. Diff Mode

- [x] 10.1 On --diff, emit `before_sha → after_sha` (or `(none) → sha`) through the output helper
- [x] 10.2 Skip diff output when `changed: false`

## 11. Tests

- [x] 11.1 Unit tests for parameter validation (missing repo/dest, relative dest, depth, SHA regex detection)
- [x] 11.2 Unit tests for version resolution with mock connector (branch, tag, SHA pass-through, default HEAD, unknown ref)
- [x] 11.3 Unit tests for repo inspection helpers
- [x] 11.4 Module tests with mock connector covering: fresh clone, update with SHA change, idempotent no-op, update:false skip, clone:false failure, dirty-fail vs dirty-force, bare flow, shallow-clone-SHA fallback warning, force=true on clean worktree (should be no-op — don't redundantly reset)
- [x] 11.5 Integration test against Docker container with git installed — fixture `tests/integration/git_playbook.yaml` using a public repo (pin small one like `https://github.com/git-fixtures/basic` or host a tiny test repo)
- [x] 11.6 Integration test for register output: downstream task consumes `after_sha`
- [x] 11.7 `go test -race ./...` passes

## 12. Documentation

- [x] 12.1 Add `docs/modules/git.md` with params table, examples (branch pin, tag pin, SHA pin, shallow, bare, recursive, key_file, accept_hostkey)
- [x] 12.2 Document SSH-only auth + target prerequisite (git binary, existing keys) + security notes on accept_hostkey
- [x] 12.3 Document known limitations: no HTTPS tokens, no LFS, Linux/macOS-only, shallow-SHA caveat, and `accept_hostkey=false` (the safe default) will fail on first connection to unknown hosts — users must pre-populate `known_hosts` or opt into `accept-new`
- [x] 12.4 Update `README.md` feature list and module table
- [x] 12.5 Update `llms.txt` with git module syntax
- [x] 12.6 Add example playbook `examples/git-deploy.yaml`
- [x] 12.7 Update `ROADMAP.md` — mark `git` module as implemented

## 13. Release

- [x] 13.1 Run `make lint` and `make test`
- [x] 13.2 Validate example playbooks with `tack validate`
- [x] 13.3 Manual smoke: clone a public repo to a Docker container and pin to a tag
