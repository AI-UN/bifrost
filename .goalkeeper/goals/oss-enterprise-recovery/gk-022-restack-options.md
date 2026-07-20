# GK-022 Restack Options

Date: `2026-05-07`

Purpose:
- Turn the remaining branch-topology blocker into a concrete execution choice.
- Map the current dirty `oss/foundation` worktree onto real `oss/*` branches without recreating a top-level `oss` branch.
- Preserve the user's `oss/*` naming requirement while minimizing risk to the already-verified restored feature set.

## Current Blocker

The functional and parity-oriented implementation work is substantially present on the current `oss/foundation` worktree, but the branch-topology success criterion is still unmet in real git state:

- `git branch --list 'oss*'` currently returns only `oss/foundation`
- no dedicated surviving local branches such as `oss/user`, `oss/adaptive-routing`, or `oss/scim-sso` exist
- the current restoration mostly lives as one large dirty worktree rather than a preserved `oss/*` branch stack

This is now a process/history problem, not a hidden feature-gap problem.

## Constraints That Still Apply

- Do not recreate a top-level `oss` branch
- Keep the `oss/*` naming style
- Preserve unrelated local changes in `.gitignore` and `AGENTS.md`
- Preserve `.goalkeeper/`
- Do not reintroduce license-gated enterprise packaging logic
- Do not disturb the already-verified parity/build state on `oss/foundation`

## Observed Change Distribution

Top-level path counts from the current dirty worktree, excluding `.goalkeeper/`:

| Area | Approx. changed paths | Notes |
|---|---:|---|
| `ui/` | 122 | Dominant surface; includes page replacements, local store decoupling, login changes, and restored workspace views |
| `transports/` | 13 tracked + many untracked handler files | Main backend API and server registration layer |
| `framework/` | 7 tracked + many untracked configstore packages | Shared persistence and migration backbone |
| `plugins/` | 4 | Adaptive routing and logging/governance integration |
| `core/` | 3 tracked + 1 untracked schema file | Shared config/runtime plumbing |
| `tests/` | 1 tracked | Placeholder parity Playwright spec |

Operational implication:
- a strict one-feature-per-branch replay is still possible, but it will require careful restaging from a very interleaved worktree
- a coarse-grained restack is lower risk because many UI and transport changes already span multiple restored features

## Option A: Strict Module Restack

This follows the original `GK-003` branch map most closely.

Suggested branches:

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

Advantages:
- best alignment with the original plan and the user's early examples such as `oss/adaptive-routing`
- clearest historical attribution by feature
- easiest future archaeology if each branch is cleanly scoped

Costs:
- highest manual restaging cost from the current interleaved worktree
- increased risk of accidentally duplicating cross-cutting files across several branches
- likely to require multiple stabilization passes because many active UI/store files now span several features at once

## Option B: Coarse-Grained Restack

This keeps `oss/*` naming while acknowledging the current worktree shape.

Suggested branches:

1. `oss/foundation`
2. `oss/governance-identity`
3. `oss/runtime-policy`
4. `oss/platform`
5. `oss/ui-restoration`

Recommended ownership:

### `oss/foundation`

Scope:
- shared configstore/migration/schema/runtime primitives that multiple restored features depend on

Representative paths:
- `framework/configstore/migrations.go`
- `framework/configstore/store.go`
- `framework/configstore/tables/framework.go`
- `framework/config.go`
- `core/schemas/bifrost.go`
- `core/schemas/envvar.go`
- `core/schemas/large_payload_config.go`
- `transports/bifrost-http/lib/config.go`
- `transports/bifrost-http/lib/config_test.go`
- `Makefile`

### `oss/governance-identity`

Scope:
- RBAC
- user groups / users / business units / access profiles
- SCIM / SSO
- teams / customers / governance compatibility surfaces
- admin API keys and browser login for admin auth

Representative paths:
- `framework/configstore/rbac*.go`
- `framework/configstore/user_groups*.go`
- `framework/configstore/sso*.go`
- `framework/configstore/admin_api_keys*.go`
- `framework/configstore/tables/rbac.go`
- `framework/configstore/tables/user_groups.go`
- `framework/configstore/tables/sso.go`
- `framework/configstore/tables/admin_api_key.go`
- `transports/bifrost-http/handlers/rbac*.go`
- `transports/bifrost-http/handlers/user_groups*.go`
- `transports/bifrost-http/handlers/sso_handler*.go`
- `transports/bifrost-http/handlers/admin_api_keys*.go`
- `transports/bifrost-http/handlers/session.go`
- `transports/bifrost-http/handlers/session_test.go`
- `transports/bifrost-http/handlers/middlewares.go`
- `ui/app/workspace/governance/**`
- `ui/app/workspace/scim/**`
- `ui/app/workspace/config/api-keys/**`
- `ui/app/login/**`
- `ui/lib/store/apis/accessProfileApi.ts`
- `ui/lib/store/apis/adminApiKeysApi.ts`
- `ui/lib/store/apis/scimApi.ts`
- `ui/lib/store/apis/userGovernanceApi.ts`
- `ui/lib/store/apis/virtualKeyUsersApi.ts`
- `ui/lib/types/accessProfile.ts`
- `ui/lib/types/user.ts`
- `ui/lib/rbac.ts`
- `ui/lib/contexts/rbacContext.tsx`

### `oss/runtime-policy`

Scope:
- audit logs
- guardrails
- PII redactor
- adaptive routing
- MCP tool groups
- alert channels
- observability connectors
- related logging/governance integration

Representative paths:
- `framework/configstore/audit*.go`
- `framework/configstore/guardrails*.go`
- `framework/configstore/pii*.go`
- `framework/configstore/adaptive_routing*.go`
- `framework/configstore/mcp_groups*.go`
- `framework/configstore/alerting*.go`
- `framework/configstore/connectors*.go`
- `transports/bifrost-http/handlers/audit_handler*.go`
- `transports/bifrost-http/handlers/guardrails*.go`
- `transports/bifrost-http/handlers/pii*.go`
- `transports/bifrost-http/handlers/adaptive_routing*.go`
- `transports/bifrost-http/handlers/mcp_groups*.go`
- `transports/bifrost-http/handlers/alert_channels*.go`
- `transports/bifrost-http/handlers/connectors*.go`
- `plugins/governance/adaptive_routing*.go`
- `plugins/governance/main.go`
- `plugins/logging/operations.go`
- `plugins/logging/utils.go`
- `ui/app/workspace/adaptive-routing/**`
- `ui/app/workspace/alert-channels/**`
- `ui/app/workspace/audit-logs/**`
- `ui/app/workspace/guardrails/**`
- `ui/app/workspace/mcp-auth-config/**`
- `ui/app/workspace/mcp-tool-groups/**`
- `ui/app/workspace/observability/views/plugins/**`
- `ui/app/workspace/pii-redactor/**`
- `ui/app/workspace/dashboard/components/userRankingsTab.tsx`
- `ui/lib/store/apis/adaptiveRoutingApi.ts`
- `ui/lib/store/apis/alertChannelsApi.ts`
- `ui/lib/store/apis/auditLogsApi.ts`
- `ui/lib/store/apis/connectorsApi.ts`
- `ui/lib/store/apis/guardrailsApi.ts`
- `ui/lib/store/apis/mcpToolGroupsApi.ts`
- `ui/lib/store/apis/piiRedactorApi.ts`

### `oss/platform`

Scope:
- cluster
- vault
- large payload
- related provider/config/runtime support

Representative paths:
- `framework/cluster/**`
- `framework/vault/**`
- `framework/configstore/vault*.go`
- `transports/bifrost-http/handlers/cluster*.go`
- `transports/bifrost-http/handlers/vault*.go`
- `transports/bifrost-http/handlers/payload*.go`
- `transports/bifrost-http/handlers/config.go`
- `transports/bifrost-http/handlers/providers.go`
- `core/providers/utils/large_response.go`
- `ui/app/workspace/cluster/**`
- `ui/app/workspace/config/views/clientSettingsView.tsx`
- `ui/app/workspace/config/views/largePayloadSettingsFragment.tsx`
- `ui/app/workspace/config/views/securityView.tsx`
- `ui/lib/store/apis/clusterApi.ts`
- `ui/lib/store/apis/largePayloadApi.ts`
- `ui/lib/store/apis/vaultApi.ts`
- `ui/lib/types/largePayload.ts`

### `oss/ui-restoration`

Scope:
- broad UI/local-store decoupling
- placeholder parity cleanup
- fallback localization
- config/dashboard/sidebar/shared workspace glue
- E2E parity coverage updates

Representative paths:
- `ui/app/_fallbacks/enterprise/**`
- `ui/app/clientLayout.tsx`
- `ui/components/sidebar.tsx`
- `ui/components/prompts/**`
- `ui/lib/store/apis/baseApi.ts`
- `ui/lib/store/apis/index.ts`
- `ui/lib/store/slices/**`
- `ui/lib/store/store.ts`
- `tests/e2e/features/placeholders/placeholders.spec.ts`

Advantages:
- materially lower risk from the current dirty worktree shape
- fewer branch boundaries across already-interleaved UI and transport files
- easier to preserve the currently verified build/parity state

Costs:
- less fine-grained history than the original branch map
- some branches will still contain several feature buckets

## Recommendation

Recommended choice: **Option B, Coarse-Grained Restack**

Reasoning:
- The current worktree is too interleaved for a low-risk strict replay without significant restaging overhead.
- The user required `oss/*` naming, but did not require reproducing the exact early branch list literally.
- The most important remaining goal risk is losing or destabilizing the already-verified restored functionality while trying to manufacture perfect branch history after the fact.
- Option B preserves meaningful `oss/*` history while keeping the restack operationally realistic.

## Safe Execution Sequence Once Approved

1. Freeze the current dirty worktree with a file-bucket manifest per planned branch.
2. Create the first target branch from the current `oss/foundation` base.
3. Stage only that branch's owned paths.
4. Commit with a branch-scoped restoration message.
5. Repeat for each planned branch in dependency order.
6. Return to `oss/foundation` and merge the resulting `oss/*` branches in order.
7. Re-run the high-signal verification set:
   - `cd transports && go test ./bifrost-http/handlers ./bifrost-http/server`
   - `cd framework && go test ./configstore/...`
   - `cd ui && npx tsc --noEmit`
   - `make build`

## Approval Gate

This document is deliberately planning-only.

Do not start rewriting branch history or manufacturing `oss/*` commit stacks from the current dirty worktree until the user explicitly chooses either:

- Option A: strict module restack
- Option B: coarse-grained restack
- or a custom variant
