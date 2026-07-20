## ADDED Requirements

### Requirement: Reachable upstream transport tag detection
The repository SHALL detect upstream transport releases by inspecting upstream tags in the `transports/v*` namespace and selecting the newest upstream transport tag whose tagged commit is reachable from the maintained fork branch.

#### Scenario: Upstream tag exists but is not yet synced
- **WHEN** the upstream repository has a newer `transports/v*` tag whose tagged commit is not an ancestor of the maintained fork branch
- **THEN** the release-sync workflow SHALL NOT create a fork release tag or trigger downstream publication for that upstream tag

#### Scenario: New synced upstream transport tag becomes reachable
- **WHEN** the maintained fork branch contains the commit referenced by a newer upstream `transports/v*` tag
- **THEN** the release-sync workflow SHALL treat that upstream tag as eligible for fork publication

### Requirement: Fork transport tag creation and state advancement
When an eligible upstream transport tag has not yet been released by the fork, the system SHALL create exactly one fork tag in the form `transports/v<upstream-version>-0` and SHALL record both the source upstream tag and the created fork tag in the sync-state file.

#### Scenario: First fork publication for an upstream transport release
- **WHEN** the newest reachable upstream tag is `transports/v1.5.2` and the sync-state file shows that this upstream tag has not yet been released by the fork
- **THEN** the workflow SHALL create the fork tag `transports/v1.5.2-0` and update the sync-state file to mark `transports/v1.5.2` as the last released upstream transport tag

#### Scenario: Workflow re-runs after fork tag already exists
- **WHEN** the fork tag for the newest reachable upstream transport release already exists
- **THEN** the workflow SHALL NOT create a second fork tag for the same upstream release

### Requirement: Explicit downstream release and Docker dispatch
After creating or rediscovering the required fork transport tag, the system SHALL explicitly dispatch the fork transport release workflow and the fork Docker publication workflow so downstream publication can complete even when the tag was pushed with `GITHUB_TOKEN`.

#### Scenario: New fork tag is created
- **WHEN** the release-sync workflow creates a new fork transport tag
- **THEN** it SHALL dispatch both downstream workflows using that fork tag as the publication ref

#### Scenario: Tag exists but downstream publication is missing
- **WHEN** the fork transport tag already exists but the GitHub release or Docker publication for that tag is missing or incomplete
- **THEN** the release-sync workflow SHALL dispatch the downstream workflows again instead of failing as a duplicate
