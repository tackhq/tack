## Why

Large playbooks become unusable without selective execution. During development, debugging, or partial re-runs, users need to run only specific tasks (e.g., just "deploy" tasks or just "config" tasks) without editing the playbook. This is a fundamental workflow gap — Ansible users expect `--tags` and `--skip-tags` as core primitives for playbook control.

## What Changes

- Add `tags:` field to tasks, blocks, plays, and role references — accepts a string or list of strings
- Add `--tags` CLI flag — only tasks matching at least one specified tag will run
- Add `--skip-tags` CLI flag — tasks matching any specified tag will be skipped
- Add special tag `always` — tasks tagged `always` run regardless of `--tags` filtering (unless explicitly skip-tagged)
- Add special tag `never` — tasks tagged `never` are skipped unless explicitly included via `--tags`
- Tag inheritance: block-level tags propagate to all tasks within the block; role-level tags propagate to all role tasks
- Tags are purely a filtering mechanism — they do not change task behavior, only whether a task executes

## Capabilities

### New Capabilities
- `tags`: Selective task execution via tag-based filtering with `--tags`/`--skip-tags` CLI flags, tag assignment on tasks/blocks/plays/roles, special `always`/`never` tags, and tag inheritance through blocks and roles

### Modified Capabilities
<!-- None -->

## Impact

- **Playbook structs** (`internal/playbook/playbook.go`): Add `Tags` field to `Task` and `Play`; add tag support to role references
- **Parser** (`internal/playbook/parser.go`): Parse `tags:` field (string or list); add to `knownTaskFields`
- **Executor** (`internal/executor/executor.go`): Add tag filtering logic before task execution; propagate inherited tags through blocks and roles
- **CLI** (`cmd/bolt/main.go`): Add `--tags` and `--skip-tags` flags; pass to executor
- **Plan mode**: Show tags in plan output; respect tag filtering in dry-run
