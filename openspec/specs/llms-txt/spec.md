## ADDED Requirements

### Requirement: LLM-optimized documentation file
The project SHALL include an `llms.txt` file at the repository root containing a self-contained reference for LLM agents to correctly generate Tack playbooks, CLI commands, and configurations.

#### Scenario: LLM generates a valid playbook
- **WHEN** an LLM agent consumes `llms.txt` and is asked to generate a Tack playbook
- **THEN** the generated playbook SHALL use correct Tack syntax (not Ansible-specific syntax)

#### Scenario: File is self-contained
- **WHEN** an LLM agent loads `llms.txt`
- **THEN** it SHALL NOT need to read any other file to generate correct Tack playbooks

### Requirement: Anti-hallucination rules
The `llms.txt` file SHALL include explicit rules that prevent LLMs from hallucinating Ansible-specific syntax, including differences in template syntax, privilege escalation keywords, and module parameters.

#### Scenario: Template syntax guidance
- **WHEN** an LLM reads the rules section
- **THEN** it SHALL find explicit instructions to use Go template syntax (`{{ .var }}`) instead of Jinja2

#### Scenario: Privilege escalation guidance
- **WHEN** an LLM reads the rules section
- **THEN** it SHALL find explicit instructions to use `sudo: true` instead of `become: true`

### Requirement: Complete module reference
The `llms.txt` file SHALL document every available module with parameter names, types, required/optional status, default values, valid enum values, and at least one example per module.

#### Scenario: Module parameter lookup
- **WHEN** an LLM needs to generate a task using the `apt` module
- **THEN** it SHALL find all apt parameters with types and valid values in `llms.txt`

### Requirement: CLI and connector reference
The `llms.txt` file SHALL document CLI commands, key flags, environment variables, and connector configuration for all supported connection types.

#### Scenario: SSH connection generation
- **WHEN** an LLM needs to generate an SSH playbook
- **THEN** it SHALL find the `ssh:` block schema, CLI flags, and env vars in `llms.txt`
