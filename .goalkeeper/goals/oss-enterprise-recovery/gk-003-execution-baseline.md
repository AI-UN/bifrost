# GK-003 Execution Baseline And Branch Map

Date: `2026-05-07`

Scope:
- Freeze the execution baseline for `oss-enterprise-recovery`.
- Define concrete `oss/<module>` branches, dependencies, and merge order.
- Translate `GK-001` and `GK-002` into an implementation sequence that can begin coding without re-litigating scope.

## Baseline State

Verified from the local repository:

| Check | Result | Meaning |
|---|---|---|
| Current branch | `oss` | Long-lived integration branch already exists |
| `upstream/main...oss` divergence | `0 0` | `oss` currently points to the same commit as `upstream/main` |
| User local modifications | `.gitignore`, `AGENTS.md` | Unrelated local edits must be preserved |
| Goal Keeper workspace | untracked `.goalkeeper/` | Planning artifacts exist locally and should remain the source of truth during execution |

Execution baseline decision:
- Treat current `oss` as the integration branch and current-main baseline.
- Do not reset or clean the working tree to create a fake pristine state.
- Start feature work from `oss`, while avoiding unrelated local files and preserving `.goalkeeper/` planning artifacts.

## Branching Rules

| Rule | Decision |
|---|---|
| Integration branch | `oss` |
| Feature branch naming | `oss/<module>` |
| Branch source | Create each feature branch from the latest `oss` after merging prior dependencies |
| Merge style | Merge completed feature branches back into `oss` incrementally after targeted verification |
| PR reference usage | Mine `pr-2565` selectively; do not cherry-pick it wholesale |
| License gating rule | Never carry `framework/license` or handler-level `requireFeature(...)` checks forward as a required OSS gating mechanism |
| Frontend rule | Do not wait for a missing `ui/app/enterprise`; restore against current OSS workspace and fallback imports |

## Concrete Branch Map

| Branch | Primary goal | Main source material | Depends on |
|---|---|---|---|
| `oss/foundation` | Common schema/migration/configstore baseline for restored enterprise features, without license gating | `framework/configstore/**`, migration updates, current main transport/config wiring | none |
| `oss/rbac` | Real RBAC backend and permission-aware UI gating | PR RBAC store/handlers/tests + current `@enterprise/lib` RBAC consumers | `oss/foundation` |
| `oss/user` | Users, teams, business units, access profiles, VK-user associations | PR user-group configstore/handlers + GK-001 user/access-profile gaps | `oss/rbac` |
| `oss/scim-sso` | SSO/SCIM configuration and auth-type integration | PR SSO store/handler + current security view and SCIM route | `oss/user` |
| `oss/audit-logs` | Immutable admin audit log APIs and audit UI | PR audit configstore/handler/tests | `oss/foundation` |
| `oss/mcp-tool-groups` | MCP tool groups and governance attachments | PR MCP groups store/handler + current MCP workspace gaps | `oss/user` |
| `oss/guardrails` | Guardrail config APIs plus runtime enforcement design on current main | PR guardrails store/handler/specs | `oss/foundation` |
| `oss/pii-redactor` | PII rules/provider overrides and logging/inference integration | PR PII store/handler/specs | `oss/guardrails` |
| `oss/adaptive-routing` | Adaptive routing config, stats, and runtime routing integration | PR adaptive routing store/handler/specs | `oss/foundation` |
| `oss/alert-channels` | Alert channel management plus runtime trigger sources | PR alerting store/handler/specs | `oss/audit-logs`, `oss/guardrails`, `oss/adaptive-routing` |
| `oss/connectors` | BigQuery/Datadog connector management and observability integration | PR connectors store/handler/specs | `oss/alert-channels` |
| `oss/cluster` | Practical OSS clustering baseline and node-health APIs | PR cluster package/handler/specs | `oss/foundation` |
| `oss/vault` | Vault-backed secret resolution and config integration | PR vault package/handler/specs | `oss/foundation` |
| `oss/large-payload` | Real large-payload config and transport/provider work | PR payload handler/specs + current client settings gap | `oss/foundation` |
| `oss/ui-restoration` | Final pass replacing placeholder routes, hybrid CTAs, and shared stubs with restored implementations | GK-001 matrix + outputs from all prior branches | all prior feature branches |

## Execution Order

Recommended sequence:

1. `oss/foundation`
2. `oss/rbac`
3. `oss/user`
4. `oss/scim-sso`
5. `oss/audit-logs`
6. `oss/mcp-tool-groups`
7. `oss/guardrails`
8. `oss/pii-redactor`
9. `oss/adaptive-routing`
10. `oss/alert-channels`
11. `oss/connectors`
12. `oss/cluster`
13. `oss/vault`
14. `oss/large-payload`
15. `oss/ui-restoration`

Why this order:
- `oss/foundation` is required before any serious backend port because `pr-2565` spreads feature storage across shared configstore and migration files.
- RBAC and user/access-profile work unblock the largest number of current fallback stubs and shared UI behaviors.
- Audit, guardrails, PII, routing, and alerting are runtime features that can be layered once the storage baseline exists.
- Cluster, Vault, and Large Payload are postponed because they have the highest architectural blast radius and the weakest direct PR completeness.
- `oss/ui-restoration` is last because current OSS UI gaps depend on APIs and data models delivered by the earlier branches.

## What Starts As Salvage vs Fresh Implementation

| Branch | Salvage-first or fresh-first | Reason |
|---|---|---|
| `oss/foundation` | salvage-first | PR has substantial configstore and table additions |
| `oss/rbac` | salvage-first | Best PR coverage across code, tests, and specs |
| `oss/user` | salvage-first | PR offers useful user-group/backend skeletons, but frontend must be fresh |
| `oss/scim-sso` | mixed | Handler/store are salvageable, but SCIM runtime and UI are fresh work |
| `oss/audit-logs` | salvage-first | Good backend reference set exists |
| `oss/mcp-tool-groups` | salvage-first | PR has storage and handler shape; frontend is fresh |
| `oss/guardrails` | mixed | PR has storage/handler code, but runtime enforcement must be fresh |
| `oss/pii-redactor` | mixed | PR has storage/handler code, but runtime redaction must be fresh |
| `oss/adaptive-routing` | mixed | PR has config/handler shape, but routing engine must be fresh |
| `oss/alert-channels` | mixed | PR has CRUD shape, but dispatch/integration must be fresh |
| `oss/connectors` | mixed | PR has CRUD shape, but actual connector behavior must be fresh |
| `oss/cluster` | mixed | PR has core package and handler, but practical OSS scope still needs fresh decisions |
| `oss/vault` | salvage-first | PR vault package is likely portable |
| `oss/large-payload` | fresh-first | PR handler is mostly placeholder behavior |
| `oss/ui-restoration` | fresh-first | PR contains no recoverable enterprise UI tree |

## First Coding Entry Point

Next coding-ready task after `GK-003`:
- `GK-004` in branch `oss/foundation`

First deliverables inside `oss/foundation`:
- current-main-compatible inventory of which PR configstore tables and migration changes can be ported directly
- decision document for replacing license gates with OSS-native enablement
- minimal shared storage and registration scaffolding required by RBAC, users, audit, MCP groups, and other follow-on branches

## GK-003 Acceptance Verdict

`complete`

Evidence:
- Current branch and divergence against `upstream/main` verified locally
- Branch map derived from `GK-001` frontend gap matrix and `GK-002` PR salvage map
- Dependencies and merge order made explicit enough to begin `oss/foundation`

Next task:
- `GK-004` port or redesign shared schema, migration, and configstore primitives.
