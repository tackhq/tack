## 1. Playbook Parsing

- [ ] 1.1 Add `VarsFiles []string` field to `Play` struct in `internal/playbook/playbook.go`
- [ ] 1.2 Parse `vars_files` from play YAML in `internal/playbook/parser.go`
- [ ] 1.3 Add `vars_files` to known play fields in validation

## 2. File Loading

- [ ] 2.1 Create `loadVarsFile(path string) (map[string]any, error)` helper in executor or vars.go
- [ ] 2.2 Implement path resolution relative to playbook directory
- [ ] 2.3 Implement optional file handling (`?` prefix — skip if missing, error otherwise)
- [ ] 2.4 Implement variable interpolation in file paths using play vars and extra-vars

## 3. Variable Merging

- [ ] 3.1 Integrate vars_files loading into `runPlayOnHost()` variable merge chain
- [ ] 3.2 Load files in order, merging each into vars (later files override)
- [ ] 3.3 Ensure precedence: play vars < vars_files < inventory vars

## 4. Testing

- [ ] 4.1 Unit test: single vars file loading and variable availability
- [ ] 4.2 Unit test: multiple files with override (last wins)
- [ ] 4.3 Unit test: relative path resolution
- [ ] 4.4 Unit test: variable interpolation in paths
- [ ] 4.5 Unit test: missing required file produces error
- [ ] 4.6 Unit test: optional file (?) skipped when missing
- [ ] 4.7 Unit test: precedence — vars_files overrides play vars, inventory overrides vars_files
