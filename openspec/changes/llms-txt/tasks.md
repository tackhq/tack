## 1. File Structure & Header

- [x] 1.1 Create `llms.txt` at repo root with version header, project identity paragraph, and "This is NOT Ansible" statement
- [x] 1.2 Write RULES section with numbered anti-hallucination rules (sudo not become, Go templates not Jinja2, `{{ var }}` not `{{ .var }}` in playbook interpolation vs `.var` in Go templates, etc.)

## 2. Playbook Schema

- [x] 2.1 Document play-level fields (name, hosts, connection, gather_facts, sudo, sudo_password, vars, vars_files, vault_file, roles, tasks, handlers, ssh, ssm) with types and defaults
- [x] 2.2 Document task-level fields (name, module, params, when, register, notify, loop, loop_var, ignore_errors, retries, delay, sudo, changed_when, failed_when) with types

## 3. Module Reference

- [x] 3.1 Document apt module: params table, valid states, one example
- [x] 3.2 Document brew module: params table, valid states, one example
- [x] 3.3 Document yum module: params table, valid states, one example
- [x] 3.4 Document command module: params table, idempotency keys, one example
- [x] 3.5 Document copy module: params table, src vs content, one example
- [x] 3.6 Document file module: params table, valid states, one example
- [x] 3.7 Document systemd module: params table, valid states, one example
- [x] 3.8 Document template module: params table, Go template syntax note, one example

## 4. CLI, Connectors & Variables

- [x] 4.1 Document CLI commands and key flags (compact table format)
- [x] 4.2 Document all 4 connectors with config schema and env vars
- [x] 4.3 Document variable system: interpolation syntax, filters, variable precedence
- [x] 4.4 Document key facts (os_family, arch, pkg_manager, ec2_*) with example values

## 5. Validation

- [x] 5.1 Review complete file for accuracy against current codebase
- [x] 5.2 Verify file is under 800 lines and fully self-contained
