# Handoff Package

## What This Goal Now Provides

This research package now defines a fact standard for the next implementation goal using official Bifrost docs as the primary source of truth.

Artifact set:

- `inventory.md`: official enterprise surface inventory
- `api-inventory.md`: grouped public API coverage
- `design-notes.md`: screenshot-backed direct observations
- `inference-notes.md`: text-only and partial-evidence reconstruction
- `api-parity.md`: public-contract to restored-code parity map
- `mismatch-notes.md`: confirmed UI/API drift against the current restored OSS branch
- `reference-specs/*.md`: implementation-ready layout contracts for major surface families
- `fallback-parity.md`: coverage audit against current demo-only fallback surfaces

## Fact Standard By Priority

### Highest Confidence

- RBAC
- SCIM / User Provisioning
- Guardrails
- Adaptive Routing
- Datadog connector

Reason:

- these surfaces have screenshot-backed layout evidence and at least some surrounding docs text

### Medium Confidence

- Audit Logs
- MCP with federated auth
- Cluster admin
- Access profiles / users / teams / business units as derived governance outcomes

Reason:

- these surfaces are supported by prose and adjacent screenshots or public API contracts, but lack full direct UI evidence

### Low Confidence / Not Confirmed By Official Enterprise Docs

- alert channels
- BigQuery connector
- large payload settings
- MCP tool groups
- PII redactor
- prompt deployments
- user rankings
- vault config

Reason:

- these modules exist on `oss/ui-restoration` or in current fallbacks, but were not confirmed by the official enterprise docs set gathered from `llms.txt`

## Recommended Module Order For The Next Implementation Goal

1. RBAC
2. SCIM / User Provisioning
3. Guardrails
4. Adaptive Routing
5. Datadog connector
6. Audit Logs
7. Cluster
8. MCP Auth Config
9. Access Profiles / Business Units / Users parity cleanup

Rationale:

- the first five modules have the strongest screenshot evidence and will set the visual/design system correctly for the weaker-evidence modules

## Contract Strategy For The Next Goal

Reuse public contracts directly where they already align:

- `/api/session/**`
- `/api/governance/**`
- `/api/logs/**`
- `/api/mcp/**`
- `/api/oauth/**`
- `/api/providers/**`
- `/api/plugins/**`

Treat these as implementation-reference private families that still need deliberate reconciliation:

- `/api/rbac/**`
- `/api/scim/auth-type`
- `/api/sso/**`
- `/api/guardrails/**`
- `/api/adaptive-routing/**`
- `/api/cluster/**`
- `/api/connectors/**`
- `/api/audit/**`
- `/api/mcp-tool-groups/**`
- `/api/user-groups/**`
- `/api/access-profiles`

## Non-Negotiable Constraints For The Next Goal

1. Official docs remain the primary evidence source.
2. Screenshot-backed layouts should override current restored OSS card layouts where they conflict.
3. Public API alignment should be preserved whenever the official contract exists.
4. Private restored contracts must be treated as temporary scaffolding until each module is checked against the docs package.
5. `unresolved` fallback surfaces from `fallback-parity.md` should not be presented as official enterprise parity targets without new evidence.

## Ready-Made Prompt Skeleton For The Next `/goal`

Use the completed research package under `.goalkeeper/goals/enterprise-docs-ui-api-parity/` as the factual standard.

Implementation rules:

- implement only the surfaces marked `covered` or `partial` in `fallback-parity.md`
- follow `reference-specs/*.md` for layout and interaction structure
- follow `api-parity.md` to distinguish public contract reuse from private restored contracts
- use `mismatch-notes.md` to replace current OSS-compatibility layouts with official-layout parity
- do not promote unresolved fallback-only modules unless new official evidence is added first
