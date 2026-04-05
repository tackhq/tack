## 1. Package Scaffold

- [ ] 1.1 Create `internal/module/git/` package with `git.go` module struct, `Name() string`, and `init()` registration
- [ ] 1.2 Implement `Describer` with all params documented in ParamDoc
- [ ] 1.3 Implement `Checker` stub for dry-run support

## 2. Parameter Parsing & Validation

- [ ] 2.1 Parse all params via `module/params.go` helpers
- [ ] 2.2 Validate `repo` is non-empty; `dest` is non-empty and absolute path
- [ ] 2.3 Validate mutual-exclusion / defaults for bools: `update` default true, `clone` default true, `force`/`bare`/`single_branch`/`recursive`/`accept_hostkey` default false
- [ ] 2.4 Validate `depth >= 0`
- [ ] 2.5 Detect whether `version` is SHA-like (`^[0-9a-f]{7,40}$`) for downstream logic

## 3. Git Binary Precheck

- [ ] 3.1 Implement `ensureGit(ctx, conn)` — runs `command -v git` (or `which git`); returns clear error when missing
- [ ] 3.2 Cache the result within a single Run (no redundant checks)

## 4. Version Resolution

- [ ] 4.1 Implement `resolveVersion(ctx, conn, repo, version, sshCmd)` — handles three cases: unset (resolve HEAD via `git ls-remote --symref`), SHA-like (pass through), else `git ls-remote <repo> <refs>`
- [ ] 4.2 Construct `GIT_SSH_COMMAND` env var from `accept_hostkey` + `key_file` and apply to ls-remote
- [ ] 4.3 Return `{resolvedSHA, resolvedRef}` or error naming unresolved ref

## 5. Repo State Inspection

- [ ] 5.1 Implement `isGitRepo(ctx, conn, dest)` — check for `.git` (worktree) or `HEAD` file (bare)
- [ ] 5.2 Implement `currentSHA(ctx, conn, dest)` — `git -C <dest> rev-parse HEAD`
- [ ] 5.3 Implement `isDirty(ctx, conn, dest)` — `git -C <dest> status --porcelain` non-empty (skip for bare)
- [ ] 5.4 Implement `currentRemoteURL(ctx, conn, dest)` — `git -C <dest> remote get-url origin`

## 6. Clone Path

- [ ] 6.1 Implement `clone(ctx, conn, params, sshCmd)` — creates parent dir with `mkdir -p`, then `git clone` with appropriate flags (`--bare`, `--depth=N`, `--single-branch --branch=<ref>`, repo, dest)
- [ ] 6.2 Run submodule update after clone when `recursive: true` and not bare
- [ ] 6.3 For fresh clones where `version` is not a branch/tag but a SHA, checkout the SHA after clone

## 7. Update Path

- [ ] 7.1 Implement `fetch(ctx, conn, dest, params, sshCmd)` — `git -C <dest> fetch origin [--depth=N]` with SHA-fetch fallback logic and warning collection
- [ ] 7.2 Implement `checkout(ctx, conn, dest, sha)` — `git -C <dest> checkout --detach <sha>` (use --detach to avoid branch-HEAD mutation surprises)
- [ ] 7.3 Run submodule update after checkout when `recursive: true` and not bare
- [ ] 7.4 Handle `force: true` via `git -C <dest> reset --hard && git -C <dest> clean -fdx` before checkout (skip for bare)

## 8. Run / Dispatch Orchestration

- [ ] 8.1 Run wiring: validate → ensureGit → if !exists && !clone: fail → if exists: currentSHA → resolveVersion → if match: return unchanged → if !update && exists: return unchanged → dirty-check → clone-or-update → post-verify SHA
- [ ] 8.2 Populate Result.Data with `{before_sha, after_sha, remote_url, version_resolved, warnings: [...]}`
- [ ] 8.3 Return `Changed(msg)` / `Unchanged(msg)` with message describing action

## 9. Check Mode

- [ ] 9.1 Implement `Check()` that performs read-only inspection + resolveVersion and returns `WouldChange`/`NoChange` without clone/fetch/checkout
- [ ] 9.2 Populate OldChecksum/NewChecksum fields with before_sha/after_sha (reuse existing CheckResult fields)

## 10. Diff Mode

- [ ] 10.1 On --diff, emit `before_sha → after_sha` (or `(none) → sha`) through the output helper
- [ ] 10.2 Skip diff output when `changed: false`

## 11. Tests

- [ ] 11.1 Unit tests for parameter validation (missing repo/dest, relative dest, depth, SHA regex detection)
- [ ] 11.2 Unit tests for version resolution with mock connector (branch, tag, SHA pass-through, default HEAD, unknown ref)
- [ ] 11.3 Unit tests for repo inspection helpers
- [ ] 11.4 Module tests with mock connector covering: fresh clone, update with SHA change, idempotent no-op, update:false skip, clone:false failure, dirty-fail vs dirty-force, bare flow, shallow-clone-SHA fallback warning
- [ ] 11.5 Integration test against Docker container with git installed — fixture `tests/integration/git_playbook.yaml` using a public repo (pin small one like `https://github.com/git-fixtures/basic` or host a tiny test repo)
- [ ] 11.6 Integration test for register output: downstream task consumes `after_sha`
- [ ] 11.7 `go test -race ./...` passes

## 12. Documentation

- [ ] 12.1 Add `docs/modules/git.md` with params table, examples (branch pin, tag pin, SHA pin, shallow, bare, recursive, key_file, accept_hostkey)
- [ ] 12.2 Document SSH-only auth + target prerequisite (git binary, existing keys) + security notes on accept_hostkey
- [ ] 12.3 Document known limitations: no HTTPS tokens, no LFS, Linux/macOS-only, shallow-SHA caveat
- [ ] 12.4 Update `README.md` feature list and module table
- [ ] 12.5 Update `llms.txt` with git module syntax
- [ ] 12.6 Add example playbook `examples/git-deploy.yaml`
- [ ] 12.7 Update `ROADMAP.md` — mark `git` module as implemented

## 13. Release

- [ ] 13.1 Run `make lint` and `make test`
- [ ] 13.2 Validate example playbooks with `tack validate`
- [ ] 13.3 Manual smoke: clone a public repo to a Docker container and pin to a tag
