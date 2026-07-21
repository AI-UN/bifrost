## ADDED Requirements

### Requirement: Scheduled and manual upstream rebase sync
The repository SHALL provide a GitHub Actions workflow that can run on a schedule and by manual dispatch to synchronize a configured fork-maintained patch branch with a configured upstream branch by rebasing fork-only commits onto the latest upstream head.

#### Scenario: Scheduled sync finds new upstream commits
- **WHEN** the scheduled sync workflow runs and the upstream branch contains commits that are not yet present in the maintained patch branch
- **THEN** the workflow rebases the fork-only commits on top of the latest upstream head and prepares the updated maintained branch for push

#### Scenario: Manual sync with no upstream delta
- **WHEN** a maintainer manually dispatches the sync workflow and the maintained patch branch already contains the latest upstream head
- **THEN** the workflow SHALL exit without rewriting the maintained branch and SHALL report that no sync was required

### Requirement: Successful sync state recording
After a successful sync, the system SHALL push the rebased maintained branch and update a tracked sync-state file with the upstream repository, upstream branch, upstream commit, and latest reachable upstream transport tag.

#### Scenario: Rebase completes successfully
- **WHEN** the sync workflow finishes a rebase without conflicts or validation failure
- **THEN** the maintained branch SHALL be updated in the fork and the sync-state file SHALL record the upstream commit and transport tag incorporated by that branch state

### Requirement: Blocked sync escalation
If the automated rebase cannot complete cleanly or required validation fails, the system SHALL preserve the attempted sync result on a timestamped diagnostic branch and SHALL maintain a single open tracking issue describing the blocked state.

#### Scenario: Rebase conflict blocks sync
- **WHEN** the sync workflow encounters a rebase conflict while replaying fork-only commits
- **THEN** the workflow SHALL push a diagnostic branch for inspection and SHALL create or update one open tracking issue with the upstream commit, maintained branch name, and failure context

#### Scenario: Later sync succeeds after a blocked run
- **WHEN** a later sync run completes successfully after a tracking issue already exists
- **THEN** the workflow SHALL close the open tracking issue because the maintained branch is back in sync
