# GK-002 PR-2565 Salvage Map

Date: `2026-05-07`

Scope:
- Classify `pr-2565` by feature bucket and portability.
- Separate salvageable backend code from useful specs/tests, missing UI, and OSS-conflicting license gates.
- Use `.goalkeeper/goals/oss-enterprise-recovery/gk-001-fallback-parity-matrix.md` as the frontend gap baseline.

## Tag Legend

| Tag | Meaning |
|---|---|
| `backend-salvageable` | PR contains meaningful backend files that can likely be ported or adapted |
| `partial-backend` | PR contains only part of the needed backend shape; runtime or integration pieces are missing |
| `specs-usable` | PR specs or design docs are useful reference material |
| `tests-usable` | PR contains tests that can be mined for contracts or verification ideas |
| `missing-ui` | PR does not provide the real UI implementation needed on current OSS |
| `stale-mismatch` | PR contains route, module, or integration mismatches that block direct reuse |
| `stubbed` | PR code exists but contains placeholder behavior rather than a complete implementation |
| `license-conflict` | PR assumes enterprise license enforcement that conflicts with the OSS restoration goal |

## Hard Evidence About What The PR Does Not Contain

| Missing tree in `pr-2565` | File count | Impact |
|---|---:|---|
| `ui/app/enterprise` | 0 | No recoverable enterprise React implementation |
| `plugins/guardrails` | 0 | Guardrails runtime plugin described in specs is absent |
| `plugins/piiredactor` | 0 | PII runtime plugin described in specs is absent |
| `plugins/adaptiverouting` | 0 | Adaptive routing runtime plugin described in specs is absent |
| `plugins/alerting` | 0 | Alert dispatch plugin described in specs is absent |
| `framework/auditstore` | 0 | Specs mention a dedicated audit store package, but code does not include it |
| `framework/scim` | 0 | Specs mention a SCIM package, but code does not include it |
| `framework/payloadstore` | 0 | Specs mention a payload store package, but code does not include it |

## Feature-By-Feature Classification

| Feature bucket | PR code artifacts present | PR tests/specs present | Missing from PR | Portability tag | Notes |
|---|---|---|---|---|---|
| RBAC | `framework/configstore/rbac.go`, `framework/configstore/tables/rbac.go`, `framework/configstore/rbac_seed.go`, `transports/bifrost-http/handlers/rbac.go`, `transports/bifrost-http/handlers/rbac_handler.go` | `tests/enterprise/rbac_test.go`, `tests/e2e/features/rbac/rbac.spec.ts`, `specs/changes/TECH-001-rbac.md` | No real UI tree | `backend-salvageable`, `specs-usable`, `tests-usable`, `missing-ui`, `license-conflict` | Strongest salvage candidate among admin features, but wrapped around license checks |
| Audit Logs | `framework/configstore/audit.go`, `framework/configstore/audit_writer.go`, `framework/configstore/tables/audit.go`, `transports/bifrost-http/handlers/audit_handler.go` | `tests/enterprise/audit_test.go`, `tests/e2e/features/audit-logs/audit-logs.spec.ts`, `specs/changes/TECH-002-audit-logs.md` | No real UI tree | `backend-salvageable`, `specs-usable`, `tests-usable`, `missing-ui`, `license-conflict` | Good backend reference set; likely portable after current-main reconciliation |
| Guardrails | `framework/configstore/guardrails.go`, `framework/configstore/tables/guardrails.go`, `transports/bifrost-http/handlers/guardrails.go` | `tests/enterprise/guardrails_test.go`, `specs/changes/TECH-003-guardrails.md` | No `plugins/guardrails`, no real UI tree | `partial-backend`, `specs-usable`, `tests-usable`, `missing-ui`, `stale-mismatch`, `license-conflict` | Specs require a runtime plugin, but PR only ships storage and handler pieces; handler routes use `/api/enterprise/guardrails/*` while tests hit `/api/guardrails/*` |
| PII Redactor | `framework/configstore/pii.go`, `framework/configstore/tables/pii.go`, `transports/bifrost-http/handlers/pii.go` | `specs/changes/TECH-004-pii-redactor.md` | No `plugins/piiredactor`, no dedicated enterprise test, no real UI tree | `partial-backend`, `specs-usable`, `missing-ui`, `license-conflict` | Data and handler layer exist, but runtime redaction plugin is absent |
| SSO / SCIM | `framework/configstore/sso.go`, `framework/configstore/tables/sso.go`, `transports/bifrost-http/handlers/sso_handler.go` | `specs/changes/TECH-005-sso-scim.md` | No `framework/scim`, no real UI tree, no dedicated SSO/SCIM test file | `partial-backend`, `specs-usable`, `missing-ui`, `stale-mismatch`, `license-conflict` | Handler covers provider/user CRUD, but spec-level SCIM package is missing; `activateUser` in `sso_handler.go` updates an SSO provider instead of an external user |
| Adaptive Routing | `framework/configstore/adaptive_routing.go`, `framework/configstore/tables/adaptive_routing.go`, `transports/bifrost-http/handlers/adaptive_routing.go` | `specs/changes/TECH-006-adaptive-routing.md` | No `plugins/adaptiverouting`, no runtime routing engine, no real UI tree | `partial-backend`, `specs-usable`, `missing-ui`, `license-conflict` | PR provides config and handler shapes, not the runtime routing logic described in specs |
| Clustering | `framework/cluster/cluster.go`, `transports/bifrost-http/handlers/cluster.go` | `specs/changes/TECH-007-clustering.md`, partial coverage via `tests/enterprise/vault_cluster_payload_test.go` | No real UI tree | `partial-backend`, `specs-usable`, `tests-usable`, `missing-ui`, `license-conflict` | Some foundational code exists, but cluster-wide behavior still needs mainline integration decisions |
| Vault | `framework/vault/client.go`, `framework/vault/json.go`, `transports/bifrost-http/handlers/vault.go` | `specs/changes/TECH-008-vault.md`, partial coverage via `tests/enterprise/vault_cluster_payload_test.go` | No real UI tree | `backend-salvageable`, `specs-usable`, `tests-usable`, `missing-ui`, `license-conflict` | Backend support package looks reusable, but frontend/config integration is absent |
| Alert Channels | `framework/configstore/alerting.go`, `framework/configstore/tables/alerting.go`, `transports/bifrost-http/handlers/alerting.go` | `specs/changes/TECH-009-alerts.md` | No `plugins/alerting`, no dedicated enterprise tests, no real UI tree | `partial-backend`, `specs-usable`, `missing-ui`, `license-conflict` | Management API exists, but async dispatch/runtime plumbing from specs is absent |
| Large Payload | `transports/bifrost-http/handlers/payload.go` | `specs/changes/TECH-010-large-payload.md`, partial coverage via `tests/enterprise/vault_cluster_payload_test.go` | No `framework/payloadstore`, no core transport/provider changes, no real UI tree | `partial-backend`, `specs-usable`, `tests-usable`, `missing-ui`, `stubbed`, `license-conflict` | Handler explicitly returns defaults and placeholder status responses rather than a real implementation |
| MCP Tool Groups | `framework/configstore/mcp_groups.go`, `framework/configstore/tables/mcp_groups.go`, `transports/bifrost-http/handlers/mcp_groups.go` | `specs/changes/TECH-011-mcp-tool-groups.md` | No real UI tree, no dedicated tests | `backend-salvageable`, `specs-usable`, `missing-ui`, `license-conflict` | Good storage and API reference, but no current PR UI or test contract |
| User Groups | `framework/configstore/user_groups.go`, `framework/configstore/tables/user_groups.go`, `transports/bifrost-http/handlers/user_groups_handler.go` | `specs/changes/TECH-012-user-groups.md` | No real UI tree, no dedicated tests | `backend-salvageable`, `specs-usable`, `missing-ui`, `license-conflict` | Useful basis for users/business units/access-profile-adjacent work |
| Data Connectors | `framework/configstore/connectors.go`, `framework/configstore/tables/connectors.go`, `transports/bifrost-http/handlers/connectors.go` | `specs/changes/TECH-013-connectors.md` | No runtime connector integration, no real UI tree, no dedicated tests | `partial-backend`, `specs-usable`, `missing-ui`, `license-conflict` | PR provides admin CRUD shape, not the actual data-flow integration implied by UI CTAs |
| License Enforcement | `framework/license/license.go`, `framework/license/errors.go`, `transports/bifrost-http/handlers/license.go` | `tests/enterprise/license_test.go`, `tests/e2e/features/license/license.spec.ts`, `specs/changes/TECH-014-license.md` | Nothing missing for its own goal | `backend-salvageable`, `tests-usable`, `specs-usable`, `license-conflict` | This is implementationally real, but strategically opposed to the OSS restoration objective and must not be carried forward as a shipping requirement |

## PR Coverage Gaps Against The GK-001 Frontend Baseline

The following current OSS enterprise gaps from `GK-001` do not have matching recoverable UI implementation in `pr-2565`:

- Full-page placeholder routes:
  - adaptive routing
  - alert channels
  - audit logs
  - cluster
  - access profiles
  - business units
  - RBAC
  - users
  - guardrails
  - MCP auth config
  - MCP tool groups
  - PII redactor
  - SCIM
- Embedded or hybrid surfaces:
  - scoped API keys CTA
  - user rankings CTA
  - prompt deployments CTA
  - BigQuery connector CTA
  - Datadog connector CTA
  - large payload fragment and API hooks

Conclusion:
- `pr-2565` is not a recoverable frontend source tree.
- It is a backend-heavy salvage branch plus extensive design material.

## Direct Reuse Boundaries

Safe to mine first:
- `framework/configstore/**` feature-specific models and store methods
- `framework/cluster/**`
- `framework/vault/**`
- `transports/bifrost-http/handlers/**` for route shape and payload contracts
- `tests/enterprise/**` for expected API semantics, after checking for drift
- `specs/changes/TECH-*.md` for feature decomposition and missing-module intent

Do not replay blindly:
- `framework/license/**`
- handler-level `requireFeature(...)` checks
- any route path or payload contract that conflicts with existing OSS handlers
- tests that target routes not matching the shipped PR handler paths

## GK-002 Acceptance Verdict

`complete`

Evidence:
- Actual changed-file list extracted from `pr-2565`
- Presence and absence of PR trees verified with `git ls-tree`
- Feature index cross-checked with `specs/changes/README.md`
- Sample handler and test files inspected to identify portability conflicts and stub behavior

Next task:
- `GK-003` write the execution baseline and feature branch map using this salvage classification and the `GK-001` frontend parity matrix.
