## ADDED Requirements

### Requirement: PLAY banner is shown only when the play has a name
The text-mode `PLAY <name>` banner SHALL be emitted if and only if the play's `name:` field is non-empty. When `name:` is empty, no PLAY line SHALL be printed; the play's host identity is conveyed by the `HOST` (single-host) or `HOSTS` (multi-host) banners that follow.

#### Scenario: Named play prints PLAY banner
- **WHEN** a play declares `name: Configure web servers`
- **THEN** the output SHALL include a line `PLAY Configure web servers` before any host banner

#### Scenario: Anonymous play omits PLAY banner
- **WHEN** a play declares no `name:` field
- **THEN** no `PLAY` line SHALL be printed; the next banner is the `HOST` line (single-host) or the `HOSTS` line (multi-host)

### Requirement: HOSTS summary line for multi-host plays
For text-mode plays targeting two or more hosts, a `HOSTS <list>` line SHALL be emitted on the main thread before per-host `HOST` banners flush. When five or fewer hosts are targeted, all host names SHALL be listed comma-separated. When more than five hosts are targeted, the first five names SHALL be listed followed by ` (and N more)` where `N = totalHosts - 5`.

#### Scenario: Three-host play lists all hosts
- **WHEN** a play targets `web1`, `web2`, `web3`
- **THEN** the output SHALL include `HOSTS web1, web2, web3`

#### Scenario: Twelve-host play truncates with overflow count
- **WHEN** a play targets twelve hosts beginning with `web1`, `web2`, `web3`, `web4`, `web5`
- **THEN** the output SHALL include `HOSTS web1, web2, web3, web4, web5 (and 7 more)`

#### Scenario: Single-host play has no HOSTS line
- **WHEN** a play targets exactly one host
- **THEN** no `HOSTS` line SHALL be emitted; the per-host `HOST` banner is sufficient

### Requirement: Fact-gathering status is inlined into the HOST banner
The text-mode HOST banner SHALL include the fact-gathering result on the same physical line. Concretely, the line SHALL render as `HOST <host> [<conn>] - gathering facts ✓` on success or `HOST <host> [<conn>] - gathering facts ✗` on failure (with the failure error continuing on a follow-up line). When `gather_facts: false`, the line SHALL render as `HOST <host> [<conn>]` with no suffix. No standalone `Gathering Facts` task line SHALL be emitted in text mode.

#### Scenario: Successful fact gather appears inline
- **WHEN** a play with default fact gathering runs against host `web1` over `ssh`
- **THEN** the output SHALL include the single line `HOST web1 [ssh] - gathering facts ✓` and SHALL NOT include a separate `Gathering Facts` task line

#### Scenario: Failed fact gather appears inline with error follow-up
- **WHEN** fact gathering fails for host `web1`
- **THEN** the output SHALL include `HOST web1 [ssh] - gathering facts ✗` followed by the gather-facts error message on a subsequent line

#### Scenario: gather_facts disabled produces a plain HOST line
- **WHEN** a play declares `gather_facts: false`
- **THEN** the output SHALL include the line `HOST <host> [<conn>]` with no fact suffix

## MODIFIED Requirements

### Requirement: Single-host fast path preserved
A play targeting exactly one host SHALL render its plan via the existing `DisplayPlan` path (no per-line host prefix, no `(N unchanged)` footer suffix). The plan body SHALL be byte-identical to current main-branch behavior; the start-of-play banners (PLAY, HOST, fact-gathering status) follow the rules in this capability and may differ from older snapshots.

#### Scenario: Single-host plan body unchanged
- **WHEN** a play targets one host with `--forks 1`
- **THEN** the rendered plan body (between the start-of-play banners and the `RECAP` line) SHALL match a snapshot of the current main-branch output exactly
