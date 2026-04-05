## Context

Tack's module system currently has two primary entry points: `Run(ctx, conn, params)` and optional `Check(ctx, conn, params)`. Both are runtime paths — they require a live connector and make decisions based on observed target state. Export is a third path: it's a **compile step** that runs entirely on the control host (ignoring connector runtime state) and produces a shell script representing what the local connector would execute.

This means every supported module needs to expose its "payload emission" logic in a way that doesn't require a running target. In practice modules already build shell commands internally (e.g., `apt-get install -y <pkg>`, `useradd -m -s /bin/bash alice`) before dispatching — export just needs to expose those strings directly.

Not every module can faithfully emit shell: `template` can render a file, but idempotency checks (compare against target file hash) require runtime state. Export's approach: emit the `write-and-backup-if-different` shell pattern, accepting that the emitted script will do its own idempotency check at run time.

## Goals / Non-Goals

**Goals:**
- Produce a standalone bash script that, when run locally on a target, reproduces what tack would do via the local connector.
- Human-readable output: one task = one commented block, faithful to the playbook's structure.
- Deterministic output: same input → byte-identical output (stable ordering, no timestamps except the banner, sorted maps).
- Explicit handling of unsupported constructs — never drop silently, always emit a `# UNSUPPORTED` comment.
- Preserve `set -euo pipefail` semantics throughout, with trap to name the failing task.
- Script is idempotent where the underlying module is idempotent.

**Non-Goals:**
- Reproduce SSH/SSM/Docker transport logic — export is local-connector payload only.
- Support every Tack feature. Handlers, rescue, register-with-runtime-downstream, async, delegate_to are all out of scope in v1; emit warnings.
- Runtime introspection — export cannot know target state, so it cannot prune tasks based on "file already exists" etc. The emitted script does runtime checks itself.
- Dry-run of the exported script — that's the user's job (`bash -n out.sh` or `bash -x out.sh`).
- Windows shell (PowerShell) export — bash only.

## Decisions

### Decision 1: Add an optional `Emitter` interface to module contract

```go
type Emitter interface {
    Emit(params map[string]any, pctx *PlayContext) (*EmitResult, error)
}

type EmitResult struct {
    Supported bool      // false → emit UNSUPPORTED comment instead
    Reason    string    // when Supported=false
    Shell     string    // bash fragment (multiline OK); must be `set -e`-safe
    PreHook   string    // optional setup emitted before the task block (deduplicated)
    Warnings  []string  // non-fatal caveats (e.g., "template mode inferred")
}
```

Modules that don't implement `Emitter` are treated as unsupported. This lets us ship export with a subset of modules working on day one, and extend module-by-module.

**Alternative:** Put a big switch-case in the export package. Rejected — keeps module-specific knowledge in the wrong place and blocks out-of-tree modules from ever supporting export.

### Decision 2: Share command construction between `Run` and `Emit`

Each module refactors its shell-building logic into pure functions: `buildInstallCmd(pkg)` etc. `Run` uses the function + connector to execute; `Emit` uses the same function to produce text. This avoids divergence (emitted shell drifting from actual runtime shell).

### Decision 3: Fact freezing via connector pre-flight

When `--no-facts` is not set, export runs `pkg/facts` against the target via the chosen connector ONCE before compilation, stores the result in-memory, and makes it available to `Emit` via `PlayContext.Facts`. Facts are also echoed into the script banner as `# FROZEN FACTS: os_type=Linux os_family=Debian ...` for audit.

When `--no-facts` is set, references like `{{ facts.os_type }}` become sentinel strings: `__TACK_FACT_NOT_GATHERED__` and the export emits a warning.

### Decision 4: `when:` evaluated at export time

Conditions evaluate against (extra vars ∪ play vars ∪ host vars ∪ frozen facts). Tasks whose `when:` resolves to false are excluded from the output with a one-line comment: `# SKIPPED (when false): <expression>`. This keeps the script lean while preserving auditability.

**Rationale:** Deferring `when:` evaluation to script runtime would require evaluating the expressions in bash — infeasible for anything beyond trivial comparisons.

**Edge case:** `when:` referencing a registered variable (which is a runtime value) → emit the task with `# WARN: when references runtime variable, included unconditionally` and keep the task.

### Decision 5: Loops expanded at export time when static

`loop:` with an explicit list or a resolvable variable → unroll into N emitted blocks. `loop:` over a dynamic / runtime-only source → emit UNSUPPORTED with reason.

Loop index variables (`item`, `ansible_loop.index`) are resolved per iteration.

### Decision 6: Tags pre-filter

If `--tags` / `--skip-tags` are set during export, tag filtering happens before emission (matching runtime behavior). Tags are also emitted as comments on surviving tasks: `# === TASK: install nginx === (tags: web,setup)`.

### Decision 7: Banner format

```
#!/usr/bin/env bash
set -euo pipefail
#
# Generated by tack export vX.Y.Z
# Playbook:   path/to/playbook.yml
# Host:       web01
# Exported:   2026-04-04T10:00:00Z
# Facts:      os_type=Linux os_family=Debian arch=x86_64 ...
#
# This is a SNAPSHOT. Re-export if playbook, inventory, or target facts change.
#
TACK_CHANGED=0
TACK_FAILED=0
TACK_CURRENT_TASK=""
on_exit() {
  local ec=$?
  if [[ $ec -ne 0 ]]; then
    echo "tack-export: FAILED on task: ${TACK_CURRENT_TASK}" >&2
  fi
  echo "tack-export: summary: changed=${TACK_CHANGED} failed=${TACK_FAILED}" >&2
  exit $ec
}
trap on_exit EXIT
```

### Decision 8: Per-task block structure

```
# === TASK: install nginx === (tags: web,setup)
TACK_CURRENT_TASK="install nginx"
<emitted shell>
TACK_CHANGED=$((TACK_CHANGED+1))  # if module reports as potentially-changing
```

For modules that may or may not change state at runtime, the `TACK_CHANGED` increment is guarded by module-specific logic (e.g., `apt-get install -y nginx` on an already-installed host won't bump changed — the emitted shell uses `apt-get -s` dry-run first or checks dpkg status). Each module's Emit defines its own change-detection shell.

### Decision 9: Determinism

- All map iteration sorts keys alphabetically before emission.
- No random IDs or timestamps embedded except the single banner timestamp.
- Facts ordering: sorted keys.
- Loop expansion preserves input list order.
- The banner timestamp is frozen to invocation start, emitted with second precision.

**Optional:** `--no-banner-timestamp` flag to omit the timestamp line for byte-identical reproducibility. Scope decision: YES, include this flag for CI/diff workflows.

### Decision 10: Unsupported-construct handling

Unsupported constructs produce:
```
# === TASK: <name> ===
# UNSUPPORTED: <reason>
# Original task YAML:
# <indented-YAML>
```

The task YAML is embedded as a comment so auditors/reviewers can see what was skipped. Export exit code: 0 if all unsupported constructs are non-fatal (explicit skip); non-zero if `--check-only` and any unsupported found.

Fatal unsupported (export fails): structural constructs that change control flow without workarounds — rescue blocks, async, delegate_to. These bail early with a clear error unless `--allow-partial` is set (v1.1 scope — not in initial release).

**v1 behavior:** Any unsupported construct simply emits the comment and continues. No `--allow-partial` flag needed.

### Decision 11: `--all-hosts` output layout

With `--all-hosts`, `--output` must be a directory. One file per host: `<dir>/<hostname>.sh`. Filename hostnames are sanitized (replace non-`[A-Za-z0-9._-]` chars with `_`). An index file `<dir>/INDEX.txt` lists the files.

With `--host <name>`, `--output` is a file path OR stdout when unset.

### Decision 12: Template rendering

`template:` module: the template is rendered AT EXPORT TIME using the current variable context, and the result is embedded in the emitted shell as a heredoc that writes the file. An idempotency guard (diff against current target content) is emitted around the heredoc.

```bash
cat > /etc/nginx/nginx.conf.tack.tmp <<'TACK_EOF'
...rendered content...
TACK_EOF
if ! diff -q /etc/nginx/nginx.conf /etc/nginx/nginx.conf.tack.tmp >/dev/null 2>&1; then
  mv /etc/nginx/nginx.conf.tack.tmp /etc/nginx/nginx.conf
  TACK_CHANGED=$((TACK_CHANGED+1))
else
  rm -f /etc/nginx/nginx.conf.tack.tmp
fi
```

Binary content is base64-embedded. Mode/owner set via chmod/chown after write.

### Decision 13: Secret handling

Vault-decrypted values DO appear in the emitted script (that's inherent — the script needs them at runtime). The export output MUST emit a prominent banner warning when any vault values are resolved:
```
# WARNING: This script contains values decrypted from vault.
# Treat this file as SECRET and do not commit to version control.
```

`no_log: true` tasks emit their commands but wrap them to suppress stdout/stderr in the script — matching the playbook's intent. The YAML comment embedding described in Decision 10 skips `no_log` tasks' param values.

### Decision 14: Roles and include_tasks

Static `include_tasks: path.yml` (no loop, no dynamic path) → recursively compile and inline. Paths resolved via existing `ResolveRolePath` logic. `include_tasks` with loop or variable-interpolated path → UNSUPPORTED. Circular include detection reuses existing executor logic.

### Decision 15: Script idempotency responsibility

The emitted script is only as idempotent as each module's emitted shell. Modules are responsible for emitting idempotent shell (check-before-change patterns). Export verifies idempotency via golden-file tests on representative playbooks + re-execution tests against Docker.

## Risks / Trade-offs

- **[Risk]** Emitted shell drifts from actual `Run` behavior → **Mitigation:** Decision 2 (shared command construction functions). Plus golden-file tests that compare Emit output to what Run would invoke.
- **[Risk]** Auditors treat the exported script as the "source of truth" and edit it directly, causing playbook drift. → **Mitigation:** Banner explicitly says "SNAPSHOT, re-export if changed." Document. Cannot be fully prevented.
- **[Risk]** Large playbooks produce very large scripts → **Mitigation:** Accepted. Consider `--split-per-play` in a future iteration.
- **[Risk]** `when:` pre-evaluation hides the condition, so reviewers don't see branching logic → **Mitigation:** Emit `# SKIPPED (when false): <expr>` for every pruned task so the decision is visible.
- **[Risk]** Facts gathered via a connector during export — requires target reachability at export time → **Mitigation:** `--no-facts` flag lets users skip this; document that some template expansions will fail without facts.
- **[Trade-off]** Adding `Emitter` to the module interface surface area → **Mitigation:** Optional interface, ship with subset of modules implementing it, add others over time.
- **[Risk]** Base64 encoding of binary copy content bloats scripts → **Mitigation:** Emit a warning when content exceeds 1MB; recommend using `copy` with external file reference + separate transport for large files (out-of-scope workaround).
- **[Trade-off]** Vault values land in plaintext in output → **Mitigation:** Prominent warning banner; document storage/handling requirements.
- **[Risk]** Module authors forget to emit idempotency guards → **Mitigation:** Module-level golden tests; CI check that runs each emitted script twice and verifies second run reports changed=0.

## Open Questions

- Should `command:` module emit `changed_when:` logic? → **Resolved:** Yes — emit the module's `changed_when:` expression as a shell conditional wrapping the change-counter bump. If unset, treat every command as potentially-changing.
- Should we emit `creates:`/`removes:` guards as shell `test -e` checks? → **Resolved:** Yes, for `command:`/`shell:` modules. Matches runtime behavior.
- Does export run inventory merging + host-vars? → **Resolved:** Yes, same pipeline as run mode; only connector interaction is swapped.
