## Why

Bolt has no way to organize variables into separate files. All variables must be inline in the playbook (`vars:` section), in inventory, or passed via `--extra-vars`. This makes multi-environment deployments cumbersome — users can't maintain separate `vars/prod.yaml` and `vars/staging.yaml` files and select between them. This is a basic workflow need that every Ansible user expects.

## What Changes

- Add `vars_files:` directive to plays, accepting a list of YAML file paths
- Files are loaded in order and merged into play variables
- File paths are relative to the playbook directory
- Support variable interpolation in file paths (e.g., `vars/{{ env }}.yaml`)
- Missing files produce an error unless marked optional
- Variable precedence: role defaults < role vars < play vars < **vars_files** < inventory vars

## Capabilities

### New Capabilities
- `vars-files`: External variable file loading with ordered merging, path interpolation, and optional file support

### Modified Capabilities

_None._

## Impact

- **Modified code**: `internal/playbook/parser.go` — parse `vars_files` field from play YAML
- **Modified code**: `internal/playbook/playbook.go` — add `VarsFiles` field to `Play` struct
- **Modified code**: `internal/executor/executor.go` — load and merge vars files during play setup
- **Modified code**: `internal/executor/vars.go` — integrate vars_files into merge chain
- **No dependency changes** — uses existing YAML parser
- **No breaking changes** — new optional field
