# Ownership Map — GK-002

Audit of current `oss/ui-restoration` endpoints and route ownership for all in-scope modules.
Generated: 2026-05-09.

---

## Module 1: RBAC

### Backend
- **Handler**: `transports/bifrost-http/handlers/rbac_handler.go` (RBACHandler)
- **Middleware**: `transports/bifrost-http/handlers/rbac.go` (RBACMiddleware)
- **Routes**:
  - `GET  /api/rbac/context` → `getContext` (authMW)
  - `GET  /api/rbac/roles` → `listRoles` (viewMW)
  - `POST /api/rbac/roles` → `createRole` (createMW)
  - `PUT  /api/rbac/roles/{role_id}` → `updateRole` (updateMW)
  - `DELETE /api/rbac/roles/{role_id}` → `deleteRole` (deleteMW)
  - `GET  /api/rbac/permissions` → `listPermissions` (viewMW)
  - `GET  /api/rbac/roles/{role_id}/permissions` → `getRolePermissions` (viewMW)
  - `PUT  /api/rbac/roles/{role_id}/permissions` → `setRolePermissions` (updateMW)
  - `GET  /api/rbac/users/{user_id}/roles` → `getUserRoles` (viewMW)
  - `PUT  /api/rbac/users/{user_id}/roles` → `setUserRoles` (updateMW)
- **Store**: `configstore.ConfigStore` → `tables.TableRole`, `tables.TablePermission`

### Frontend
- **API slice**: `ui/lib/store/apis/rbacApi.ts`
- **Types**: `ui/lib/types/rbac.ts`
- **RTK hooks**: `useGetRbacContextQuery`, `useGetRolesQuery`, `useCreateRoleMutation`, `useUpdateRoleMutation`, `useDeleteRoleMutation`, `useGetPermissionsQuery`, `useGetRolePermissionsQuery`, `useUpdateRolePermissionsMutation`, `useGetUserRolesQuery`, `useUpdateUserRolesMutation`

### Routes
- **Primary**: `ui/app/workspace/governance/rbac/rbacView.tsx` (the actual view)
- **Redirect**: `ui/app/workspace/rbac/page.tsx` → redirects to `/workspace/governance/rbac`
- **Sidebar link**: `ui/components/sidebar.tsx:785` → `/workspace/governance/rbac`

### Auth/Session Dependencies
- `/api/rbac/context` requires a resolved principal (session token or local admin)
- The `RBACMiddleware.ResolvePrincipal()` checks: external user ID → local admin → session token
- **KNOWN RISK**: `/api/rbac/context` has previously caused 401 auth-loop. Must test after changes.

### Contract Status
- `private-only` — no public `/api-reference/rbac/**` family
- Closest public: `/api/users/me/permissions`

---

## Module 2: SCIM / User Provisioning

### Backend
- **Handler**: `transports/bifrost-http/handlers/sso_handler.go` (SSOHandler)
- **Routes**:
  - `GET  /api/scim/auth-type` → `getAuthType` (settingsViewMW)
  - `GET  /api/sso/providers` → `listProviders` (userProvisioningViewMW)
  - `POST /api/sso/providers` → `createProvider` (userProvisioningCreateMW)
  - `GET  /api/sso/providers/{provider_id}` → `getProvider` (userProvisioningViewMW)
  - `PUT  /api/sso/providers/{provider_id}` → `updateProvider` (userProvisioningUpdateMW)
  - `DELETE /api/sso/providers/{provider_id}` → `deleteProvider` (userProvisioningDeleteMW)
  - `POST /api/sso/providers/{provider_id}/test` → `testProvider` (userProvisioningViewMW)
  - `GET  /api/sso/users` → `listUsers` (userProvisioningViewMW)
  - `GET  /api/sso/users/{user_id}` → `getUser` (userProvisioningViewMW)
  - `POST /api/sso/users/{user_id}/deactivate` → `deactivateUser` (userProvisioningUpdateMW)
  - `POST /api/sso/users/{user_id}/activate` → `activateUser` (userProvisioningUpdateMW)
- **Store**: `configstore.ConfigStore`

### Frontend
- **API slice**: `ui/lib/store/apis/scimApi.ts`
- **Types**: `AuthTypeResponse`, `SSOProvider`, `ExternalSSOUser` (inline in slice)
- **RTK hooks**: `useGetAuthTypeQuery`, `useGetSSOProvidersQuery`, `useCreateSSOProviderMutation`, `useUpdateSSOProviderMutation`, `useDeleteSSOProviderMutation`, `useTestSSOProviderMutation`, `useGetExternalSSOUsersQuery`, `useDeactivateExternalSSOUserMutation`, `useActivateExternalSSOUserMutation`

### Routes
- **Primary**: `ui/app/workspace/scim/scimView.tsx` (231 lines)
- **Sidebar link**: `ui/components/sidebar.tsx:778` → `/workspace/scim`

### Auth/Session Dependencies
- All SSO routes require RBAC middleware for "User Provisioning" resource

### Contract Status
- `private-only` — no public `/api-reference/sso/**` or `/api-reference/scim/**`

---

## Module 3: Guardrails

### Backend
- **Handler**: `transports/bifrost-http/handlers/guardrails.go` (GuardrailsHandler)
- **Middleware**: `transports/bifrost-http/handlers/guardrails_middleware.go`
- **Routes**:
  - `GET  /api/guardrails/policies` → `listPolicies` (configViewMW)
  - `POST /api/guardrails/policies` → `createPolicy` (configCreateMW)
  - `GET  /api/guardrails/policies/{id}` → `getPolicy` (configViewMW)
  - `PUT  /api/guardrails/policies/{id}` → `updatePolicy` (configUpdateMW)
  - `DELETE /api/guardrails/policies/{id}` → `deletePolicy` (configDeleteMW)
  - `GET  /api/guardrails/policies/{id}/rules` → `listRules` (configViewMW)
  - `POST /api/guardrails/policies/{id}/rules` → `createRule` (configCreateMW)
  - `DELETE /api/guardrails/rules/{ruleId}` → `deleteRule` (configDeleteMW)
  - `GET  /api/guardrails/violations` → `queryViolations` (providersViewMW)
  - `POST /api/guardrails/test` → `testGuardrails` (providersViewMW)
- **Store**: `configstore.ConfigStore`

### Frontend
- **API slice**: `ui/lib/store/apis/guardrailsApi.ts`
- **Types**: `GuardrailPolicy`, `GuardrailRule`, `GuardrailViolation` (inline)
- **RTK hooks**: `useGetGuardrailPoliciesQuery`, `useGetGuardrailPolicyQuery`, `useCreateGuardrailPolicyMutation`, `useUpdateGuardrailPolicyMutation`, `useDeleteGuardrailPolicyMutation`, `useCreateGuardrailRuleMutation`, `useDeleteGuardrailRuleMutation`, `useGetGuardrailViolationsQuery`, `useTestGuardrailsMutation`

### Routes
- **Configuration**: `ui/app/workspace/guardrails/configuration/guardrailsConfigurationView.tsx` (372 lines)
- **Providers**: `ui/app/workspace/guardrails/providers/guardrailsProviderView.tsx`
- **Sidebar links**: `/workspace/guardrails` (parent), `/workspace/guardrails/configuration`, `/workspace/guardrails/providers`

### Contract Status
- `private-only` — no public guardrails contract family

---

## Module 4: Adaptive Routing

### Backend
- **Handler**: `transports/bifrost-http/handlers/adaptive_routing.go` (AdaptiveRoutingHandler)
- **Routes**:
  - `GET  /api/adaptive-routing/policies` → `listPolicies` (viewMW)
  - `POST /api/adaptive-routing/policies` → `createPolicy` (createMW)
  - `GET  /api/adaptive-routing/policies/{id}` → `getPolicy` (viewMW)
  - `PUT  /api/adaptive-routing/policies/{id}` → `updatePolicy` (updateMW)
  - `DELETE /api/adaptive-routing/policies/{id}` → `deletePolicy` (deleteMW)
  - `GET  /api/adaptive-routing/metrics` → `listMetrics` (viewMW)
  - `POST /api/adaptive-routing/metrics/refresh` → `refreshMetrics` (updateMW)
  - `GET  /api/adaptive-routing/quality-scores` → `listQualityScores` (viewMW)
  - `PUT  /api/adaptive-routing/quality-scores` → `upsertQualityScore` (updateMW)
  - `DELETE /api/adaptive-routing/quality-scores` → `deleteQualityScore` (deleteMW)
- **Store**: `configstore.ConfigStore`

### Frontend
- **API slice**: `ui/lib/store/apis/adaptiveRoutingApi.ts`
- **Types**: `AdaptiveRoutingPolicy`, `AdaptiveRoutingMetric`, `AdaptiveRoutingQualityScore` (inline)
- **RTK hooks**: various CRUD + metrics + quality scores

### Routes
- **Primary**: `ui/app/workspace/adaptive-routing/adaptiveRoutingView.tsx` (751 lines)
- **Sidebar link**: `ui/components/sidebar.tsx:838` → `/workspace/adaptive-routing`

### Contract Status
- `private-only` — closest public anchor: `/api/governance/routing-rules`

---

## Module 5: Datadog Connector

### Backend
- **Handler**: `transports/bifrost-http/handlers/connectors.go` (ConnectorsHandler)
- **Routes**:
  - `GET  /api/connectors` → `listConnectors` (viewMW)
  - `POST /api/connectors` → `createConnector` (createMW)
  - `GET  /api/connectors/{id}` → `getConnector` (viewMW)
  - `PUT  /api/connectors/{id}` → `updateConnector` (updateMW)
  - `DELETE /api/connectors/{id}` → `deleteConnector` (deleteMW)
  - `POST /api/connectors/{id}/test` → `testConnector` (updateMW)
- **Store**: `configstore.ConfigStore`

### Frontend
- **API slice**: `ui/lib/store/apis/connectorsApi.ts`
- **Types**: `Connector` (generic, config is `Record<string, unknown>`)
- **RTK hooks**: `useGetConnectorsQuery`, `useCreateConnectorMutation`, `useUpdateConnectorMutation`, `useDeleteConnectorMutation`, `useTestConnectorMutation`

### Routes
- **Primary**: `ui/app/workspace/observability/views/plugins/datadogView.tsx` (24 lines — thin wrapper over `ConnectorConfigView`)
- **Generic wrapper**: `ui/app/workspace/observability/views/plugins/connectorConfigView.tsx`
- **Sidebar**: `/workspace/observability` (Observability → Connectors)

### Contract Status
- `private-only` — generic connector CRUD, Datadog is a specific connector `type`

---

## Module 6: Audit Logs

### Backend
- **Handler**: `transports/bifrost-http/handlers/audit_handler.go` (AuditHandler)
- **Routes**:
  - `GET  /api/audit/logs` → `queryLogs` (auditLogsViewMW)
  - `POST /api/audit/verify` → `verifyChain` (auditLogsViewMW)
- **Store**: `configstore.ConfigStore`

### Frontend
- **API slice**: `ui/lib/store/apis/auditLogsApi.ts`
- **Types**: `AuditLogEntry` (inline)
- **RTK hooks**: `useGetAuditLogsQuery`, `useVerifyAuditChainMutation`

### Routes
- **Primary**: `ui/app/workspace/audit-logs/auditLogsView.tsx`
- **Sidebar link**: `ui/components/sidebar.tsx:799` → `/workspace/audit-logs`

### Contract Status
- `private-only` — no public audit contract family

---

## Module 7: Cluster

### Backend
- **Handler**: `transports/bifrost-http/handlers/cluster.go` (ClusterHandler)
- **Routes**:
  - `GET  /api/cluster/status` → `getStatus` (viewMW)
  - `POST /api/cluster/drain` → `requestDrain` (updateMW)
- **Store**: `configstore.ConfigStore`

### Frontend
- **API slice**: `ui/lib/store/apis/clusterApi.ts`
- **Types**: `ClusterNode`, `ClusterStatus` (inline)
- **RTK hooks**: `useGetClusterStatusQuery`, `useRequestClusterDrainMutation`

### Routes
- **Primary**: `ui/app/workspace/cluster/clusterView.tsx`
- **Sidebar link**: `ui/components/sidebar.tsx:831` → `/workspace/cluster`

### Contract Status
- `private-only`

---

## Module 8: MCP Auth Config

### Backend
- **Assembled from**: `transports/bifrost-http/handlers/mcp.go`, `oauth2.go`, `oauth2_per_user.go`
- **No dedicated handler** — the page consumes MCP client data via existing MCP API

### Frontend
- **API slice**: `ui/lib/store/apis/mcpApi.ts` (shared with MCP registry)
- **RTK hooks**: `useGetMCPClientsQuery` (reused)

### Routes
- **Primary**: `ui/app/workspace/mcp-auth-config/mcpAuthConfigView.tsx`
- **Sidebar**: NOT directly linked in sidebar — likely accessed via MCP registry or governance section
- **Note**: `mcp-auth-config` is not found in sidebar.tsx grep. It may be accessible through internal navigation only.

### Contract Status
- `partial-public` — MCP + OAuth public contracts exist but no dedicated auth-config page

---

## Module 9: Access Profiles

### Backend
- **Handler**: `transports/bifrost-http/handlers/user_groups.go` (UserGroupHandler)
- **Routes**:
  - `GET  /api/access-profiles` → `listAccessProfiles` (accessProfilesViewMW)
  - `GET  /api/users/{user_id}/access-profiles` → `listUserAccessProfiles` (virtualKeysViewMW)
- **Store**: `configstore.ConfigStore`

### Frontend
- **API slice**: `ui/lib/store/apis/accessProfileApi.ts`
- **Types**: `ui/lib/types/accessProfile.ts`
- **RTK hooks**: `useGetAccessProfilesQuery`, `useGetUserAccessProfilesQuery`

### Routes
- **Primary**: `ui/app/workspace/governance/access-profiles/accessProfilesView.tsx`
- **Sidebar link**: `ui/components/sidebar.tsx:792` → `/workspace/governance/access-profiles`

---

## Module 10: Users

### Backend
- **Handler**: `transports/bifrost-http/handlers/user_groups.go` (same as Access Profiles)
- **Routes**:
  - `GET  /api/users` → `listUsers` (usersViewMW)

### Frontend
- **API slice**: `ui/lib/store/apis/userGovernanceApi.ts` (`useGetUsersQuery`)
- **Types**: `ui/lib/types/user.ts`

### Routes
- **Primary**: `ui/app/workspace/governance/users/usersView.tsx`
- **Sidebar link**: `ui/components/sidebar.tsx:750` → `/workspace/governance/users`

---

## Module 11: Teams

### Backend
- **Handler**: `transports/bifrost-http/handlers/governance.go` (teams CRUD via governance) + `user_groups.go`
- **Routes** (governance): `GET/POST/PUT/DELETE /api/governance/teams`
- **Routes** (user_groups): team-related operations via `/api/user-groups/**`

### Frontend
- **API slice**: `ui/lib/store/apis/governanceApi.ts` (governance teams)
- **Types**: `ui/lib/store/apis/userGovernanceApi.ts` has `BusinessUnit` but teams come from governance

### Routes
- **Primary**: `ui/app/workspace/governance/teams/teamsView.tsx`
- **Sidebar link**: `ui/components/sidebar.tsx:757` → `/workspace/governance/teams`

---

## Module 12: Business Units

### Backend
- **Handler**: `transports/bifrost-http/handlers/user_groups.go`
- **Routes**:
  - `GET/POST/PUT/DELETE /api/user-groups` and `/api/user-groups/{group_id}`

### Frontend
- **API slice**: `ui/lib/store/apis/userGovernanceApi.ts` (`useGetBusinessUnitsQuery` — maps `/api/user-groups` response)
- **Types**: `BusinessUnit` (inline in userGovernanceApi)

### Routes
- **Primary**: `ui/app/workspace/governance/business-units/businessUnitsView.tsx`
- **Sidebar link**: `ui/components/sidebar.tsx:764` → `/workspace/governance/business-units`

---

## Cross-Cutting Observations

1. **No ContactUsView references found** in any workspace views — the fallback/demo mode has already been removed from these routes.
2. **`mcp-auth-config` is not in sidebar** — only accessible via direct navigation or internal links.
3. **Session API** (`/api/session/**`) is the auth foundation for all routes — checked via `sessionApi.ts`.
4. **RBAC middleware** gates most routes via resource+operation pairs. The RBAC handler itself uses "RBAC" resource; SSO uses "User Provisioning"; Guardrails uses "Config"; etc.
5. **Dual team paths**: governance teams (`/api/governance/teams`) vs user-groups (`/api/user-groups`) — reconciliation needed for Module 11.
6. **All in-scope modules already have working backend handlers and frontend API slices** — this is a UI structure/parity alignment task, not a greenfield build.
