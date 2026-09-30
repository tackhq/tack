## ADDED Requirements

### Requirement: Playbook-level `become` default
In the mapping format, the playbook-level defaults SHALL accept `become` as an exact alias of `sudo`, with identical inheritance semantics. Setting both `sudo` and `become` at the playbook level with different values SHALL be a parse error.

#### Scenario: Playbook-level become inherits
- **WHEN** the playbook mapping declares `become: true` and a play omits `sudo`/`become`
- **THEN** the play's effective `Sudo` is `true`

#### Scenario: Conflicting playbook-level keys
- **WHEN** the playbook mapping declares `sudo: true` and `become: false`
- **THEN** parsing fails with an error naming both keys
