## ADDED Requirements

### Requirement: Approval prompt identifies the target host(s)
The interactive approval prompt SHALL include a description of the target host(s) and connection type so the user can identify what is about to be modified without scrolling above the prompt. The prompt text SHALL take the form `Apply these changes to <target>? (yes/no): ` where `<target>` is rendered as follows:

- For a play targeting exactly one host, `<target>` SHALL be `<host> (<connection>)`. Example: `web1.prod (ssh)` or `i-0a1b2c3d4e5f (ssm)`.
- For a play targeting two or more hosts, `<target>` SHALL begin with the host count and SHALL list host names in parentheses. When the host count is five or fewer, all names SHALL be listed. When the host count exceeds five, the first five names SHALL be listed followed by a literal `, ...` suffix. Example: `4 hosts (web1, web2, web3, web4)`; `12 hosts (web1, web2, web3, web4, web5, ...)`.

The prompt content rule SHALL apply to both the single-host fast path (after `DisplayPlan`) and the consolidated multi-host path (after `DisplayMultiHostPlan`). The `--auto-approve` flag and the JSON emitter's auto-approval behavior SHALL be unaffected: in those modes no prompt is shown.

#### Scenario: Single-host SSH play
- **WHEN** a play targets exactly one host `web1.prod` over `connection: ssh` and `--auto-approve` is not set
- **THEN** the prompt line SHALL read `Apply these changes to web1.prod (ssh)? (yes/no): `

#### Scenario: Single-host SSM instance
- **WHEN** a play targets exactly one host `i-0a1b2c3d4e5f` over `connection: ssm`
- **THEN** the prompt line SHALL read `Apply these changes to i-0a1b2c3d4e5f (ssm)? (yes/no): `

#### Scenario: Multi-host play within the visible cap
- **WHEN** a play targets four hosts `web1`, `web2`, `web3`, `web4`
- **THEN** the prompt line SHALL read `Apply these changes to 4 hosts (web1, web2, web3, web4)? (yes/no): `

#### Scenario: Multi-host play exceeding the visible cap
- **WHEN** a play targets twelve hosts beginning with `web1`, `web2`, `web3`, `web4`, `web5`
- **THEN** the prompt line SHALL read `Apply these changes to 12 hosts (web1, web2, web3, web4, web5, ...)? (yes/no): `

#### Scenario: Auto-approve suppresses the prompt
- **WHEN** `--auto-approve` is set on any play
- **THEN** no prompt SHALL be shown and the executor SHALL proceed to apply

#### Scenario: JSON emitter does not prompt
- **WHEN** the JSON emitter is active
- **THEN** the emitter SHALL auto-approve regardless of the host target string and SHALL NOT print the prompt to stdout
