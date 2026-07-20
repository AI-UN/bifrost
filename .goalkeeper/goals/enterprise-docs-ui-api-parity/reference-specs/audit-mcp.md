# Audit And MCP Reference Spec

## Evidence Basis

- `enterprise/audit-logs.md`
- `enterprise/mcp-with-fa.md`
- public API docs for logging, MCP, OAuth, and per-user OAuth

## Confidence Warning

These surfaces are materially less certain than RBAC, SCIM, Guardrails, Adaptive Routing, and Datadog because the official enterprise docs do not provide equivalent screenshot-backed product pages.

## Surface A: Audit Logs

### Direct Facts

- the enterprise docs position audit logs as immutable security/compliance records
- audit evidence is distinct from request logs and MCP tool logs
- integrity verification is part of the enterprise narrative

### Recommended Layout Direction

Inference:

- primary screen should be an append-only log table
- integrity or chain-verification actions should be visible near the page header
- record fields should emphasize actor, action, target, timestamp, and verification status

### Contract Notes

- public docs do not expose a dedicated audit contract family
- restored branch uses:
  - `/api/audit/logs`
  - `/api/audit/verify`

## Surface B: MCP Auth / Federated Auth Admin

### Direct Facts

- official docs discuss MCP tools backed by federated or per-user auth
- public docs do expose MCP client registry and OAuth lifecycle contracts

### Recommended Layout Direction

Inference:

- this should be treated as an admin surface over client auth modes
- counts, status, and entry points into OAuth completion flows are valid
- but a dedicated configuration editor should not be assumed unless new screenshot evidence appears

### Contract Notes

- public contracts available:
  - `/api/mcp/**`
  - `/api/oauth/**`
  - `/api/oauth/per-user/**`
- private restored surface:
  - `/api/mcp-tool-groups/**`

## Acceptance Checks For Future Implementation

1. Keep confidence labels visible in implementation planning for these modules.
2. Prefer explicit “docs say X / UI inferred as Y” annotations in specs or tickets.
3. Do not overfit the restored OSS pages into “official” status where screenshot evidence is absent.
