## 1. Parser: detect mapping vs. sequence at root

- [x] 1.1 In `internal/playbook/parser.go`, replace the current dual-unmarshal entry point with a `yaml.Node` decode and switch on `node.Kind` (sequence vs. mapping).
- [x] 1.2 Sequence path: keep existing behavior — decode each item as a play with no defaults.
- [x] 1.3 Mapping path: if the mapping has a `plays:` key, parse playbook-level defaults (`hosts`, `connection`, `sudo`, `vars`) and decode `plays:` as `[]map[string]any`.
- [x] 1.4 Mapping path without `plays:`: preserve current "single play as map at root" fallback (no defaults applied).
- [x] 1.5 Reuse the existing scalar-or-list logic for the playbook-level `hosts:` field (string and `[]any` both accepted).
- [x] 1.6 Return a clear error if `plays:` is present but not a sequence, or if any reserved playbook-level key has the wrong type.

## 2. Apply defaults at parse time

- [x] 2.1 Add `PlaybookDefaults` struct in `internal/playbook/playbook.go` with `Hosts []string`, `Connection string`, `Sudo bool`, `Vars map[string]any`.
- [x] 2.2 Add `Defaults *PlaybookDefaults` field on `Playbook` (informational; merged values are already on each play).
- [x] 2.3 After `parseRawPlay` for each play, fill in unset fields from defaults: `Hosts` if empty, `Connection` if empty string, `Sudo` if defaults.Sudo is true and play didn't set it true. (Document that playbook-level `sudo: false` is a no-op.)
- [x] 2.4 Merge `vars`: start from a copy of `defaults.Vars`, then overlay play-level `vars` so play keys win.
- [x] 2.5 Confirm SSH, SSM, VarsFiles, VaultFile, Roles, Handlers, Tasks are NOT inherited (out of scope per design).

## 3. Validation

- [x] 3.1 Update `Play.Validate()` error message for missing hosts to mention the playbook-level option (e.g. "play has no hosts; declare `hosts:` on the play or at the playbook level").
- [x] 3.2 Confirm `tack validate` works for both formats end-to-end.

## 4. Tests

- [x] 4.1 Add `parser_test.go` cases covering: sequence format unchanged, mapping format with all defaults, mapping format with plays overriding each default field, mapping format with vars merge precedence, mapping without `plays:` errors, malformed `plays:` (non-sequence) errors.
- [x] 4.2 Add a `Play.Validate()` test for the new error message when hosts are missing at both levels.
- [x] 4.3 Add an executor-level smoke test (or extend an existing one) confirming a mapping-format playbook with `hosts: webservers` resolves identically to the equivalent sequence-format playbook against the same inventory.
- [x] 4.4 Run `make test` and `make lint`; fix any regressions.

## 5. Examples and docs

- [x] 5.1 Add `examples/playbook-defaults.yaml` showing the mapping format with `hosts:`, `connection:`, and 2–3 plays.
- [x] 5.2 Update `docs/getting-started.md` with a brief section "Playbook-level defaults" pointing at the new example.
- [x] 5.3 Update `README.md` (Playbook Structure or equivalent section) to mention both formats.
- [x] 5.4 Update `llms.txt` if it enumerates the playbook structure.
- [x] 5.5 Re-run `tack validate examples/*.yaml` to confirm all examples (old and new) still parse.
