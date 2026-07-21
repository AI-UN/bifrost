# GK-025 Restack Runbook

Date: `2026-05-07`

Purpose:
- Turn the approved coarse-grained `oss/*` restack plan into an execution playbook.
- Provide a deterministic order of operations once the user authorizes restack execution.
- Reduce restack risk by inserting verification and safety checkpoints between branch steps.

Preconditions:
- User explicitly approves the coarse-grained restack variant.
- Do not recreate a top-level `oss` branch.
- Preserve `.goalkeeper/**`, `.gitignore`, and `AGENTS.md` outside the feature-restack commits.

Related artifacts:
- [gk-022-restack-options.md](/home/evans/Projects/Public/bifrost/.goalkeeper/goals/oss-enterprise-recovery/gk-022-restack-options.md)
- [gk-023-restack-manifest.md](/home/evans/Projects/Public/bifrost/.goalkeeper/goals/oss-enterprise-recovery/gk-023-restack-manifest.md)
- [gk-024-completion-checklist.md](/home/evans/Projects/Public/bifrost/.goalkeeper/goals/oss-enterprise-recovery/gk-024-completion-checklist.md)

## Target Branch Line

1. `oss/foundation`
2. `oss/governance-identity`
3. `oss/runtime-policy`
4. `oss/platform`
5. `oss/ui-restoration`

## Safety Rules

- Do not stage `.goalkeeper/**`, `.gitignore`, or `AGENTS.md`.
- Do not delete or rewrite the current dirty worktree before a branch-specific patch has been captured.
- After each branch commit, record the exact staged paths and any hunk-split files used.
- If a shared file cannot be cleanly split by intent, defer that file to `oss/ui-restoration` unless it blocks a backend branch from building.

## Recommended High-Level Method

Use a patch-capture workflow rather than trying to hand-edit the working tree repeatedly:

1. Create a full backup patch of the current dirty worktree.
2. Materialize one branch at a time from `oss/foundation`.
3. For each target branch:
   - restore the full patch into the worktree
   - stage only that branch's owned files/hunks from `gk-023`
   - commit
   - reset the worktree back to the preserved dirty baseline before moving to the next branch
4. Merge the resulting branches back into `oss/foundation` in dependency order.

Why this method:
- it keeps the source of truth as the current verified dirty worktree
- it avoids losing interleaved changes during repeated staging cycles
- it minimizes accidental omission versus trying to manually rebuild each branch from memory

## Step-By-Step Runbook

### 0. Baseline Capture

Commands:

```bash
git status --short --branch
git branch --list 'oss*'
git diff > /tmp/bifrost-oss-restack-working.patch
git diff --staged > /tmp/bifrost-oss-restack-index.patch
git ls-files --others --exclude-standard > /tmp/bifrost-oss-restack-untracked.txt
```

Checkpoint:
- confirm the patch files and untracked-file manifest exist before any branch surgery begins

### 1. Foundation Branch Commit

Intent:
- capture only shared build/runtime/configstore scaffolding from `gk-023`

Steps:

```bash
git checkout oss/foundation
git checkout -b oss/foundation-restack
```

Then:
- apply the saved dirty patch if needed
- stage only the `oss/foundation` files from `gk-023`
- confirm staged paths with:

```bash
git diff --cached --name-only
```

Commit suggestion:

```text
restore(oss): add shared foundation scaffolding for enterprise recovery
```

### 2. Governance Identity Branch

Branch source:
- from the committed foundation branch

Steps:

```bash
git checkout -b oss/governance-identity
```

Then:
- restore the preserved dirty patch to the worktree if needed
- stage only governance-identity files/hunks from `gk-023`
- verify:

```bash
git diff --cached --name-only
```

Suggested targeted verification after commit:

```bash
cd transports && go test ./bifrost-http/handlers -run 'TestUserGroupCompatRoute_|TestSSOHandler|TestAdminAPIKeys|TestRBAC'
cd ui && npx tsc --noEmit
```

Commit suggestion:

```text
restore(oss): recover governance identity and admin auth surfaces
```

### 3. Runtime Policy Branch

Branch source:
- from `oss/governance-identity`

Steps:

```bash
git checkout -b oss/runtime-policy
```

Then:
- restore the preserved dirty patch to the worktree if needed
- stage runtime-policy files/hunks from `gk-023`

Suggested targeted verification after commit:

```bash
cd framework && go test ./configstore/...
cd transports && go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations
cd plugins/governance && go test ./...
cd ui && npx tsc --noEmit
```

Commit suggestion:

```text
restore(oss): recover runtime policy and observability management surfaces
```

### 4. Platform Branch

Branch source:
- from `oss/runtime-policy`

Steps:

```bash
git checkout -b oss/platform
```

Then:
- restore the preserved dirty patch to the worktree if needed
- stage platform files/hunks from `gk-023`

Suggested targeted verification after commit:

```bash
cd framework && go test ./configstore/... ./vault/... ./cluster/...
cd transports && go test ./bifrost-http/handlers ./bifrost-http/server
cd ui && npx tsc --noEmit
```

Commit suggestion:

```text
restore(oss): recover platform and large-payload features
```

### 5. UI Restoration Branch

Branch source:
- from `oss/platform`

Steps:

```bash
git checkout -b oss/ui-restoration
```

Then:
- restore the preserved dirty patch to the worktree if needed
- stage the remaining UI/store/fallback-localization files from `gk-023`

Suggested targeted verification after commit:

```bash
cd ui && npx tsc --noEmit
cd tests/e2e && npx playwright test features/placeholders/placeholders.spec.ts --list
```

Commit suggestion:

```text
restore(oss): localize fallback UI and finalize OSS parity surfaces
```

### 6. Merge Back Into `oss/foundation`

Once the branch line exists:

```bash
git checkout oss/foundation
git merge --no-ff oss/foundation-restack
git merge --no-ff oss/governance-identity
git merge --no-ff oss/runtime-policy
git merge --no-ff oss/platform
git merge --no-ff oss/ui-restoration
```

If the user wants the original branch name preserved more strictly:
- rename `oss/foundation-restack` back to `oss/foundation` only after clarifying how to avoid clobbering the current dirty branch state

## Final Verification Gate

After the merge sequence:

```bash
git branch --list 'oss*'
cd framework && go test ./configstore/...
cd transports && go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations
cd ui && npx tsc --noEmit
make build
```

Optional stronger rerun if environment is available:

```bash
cd tests/e2e && npx playwright test features/placeholders/placeholders.spec.ts
```

## Completion Gate

The goal may only be marked complete after all of the following are true:

- real `oss/*` branches exist beyond `oss/foundation`
- the restored code still builds
- the parity checklist remains valid after the restack
- no required restored feature has been dropped during branch splitting

## Runbook Verdict

This runbook is sufficient to begin the actual `oss/*` restack immediately once the user authorizes the coarse-grained restack option.
