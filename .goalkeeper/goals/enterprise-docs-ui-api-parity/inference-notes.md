# Text-Only And Partial-Evidence Inference Notes

## Reading Rule

- `Observed from docs` means the statement is directly supported by official enterprise prose or code/config examples in the docs.
- `Inference for next goal` means the statement is a reconstruction hypothesis for later implementation work.
- This file intentionally does not claim pixel-perfect UI details where screenshots are missing.

## Advanced Governance

### Observed from docs

1. The page is titled `Getting started` but describes `Advanced governance features with enhanced security, compliance reporting, audit trails, and enterprise-grade access controls for large-scale deployments`.
2. Official text says enterprise governance extends core governance with:
   identity and access management,
   user-level governance,
   RBAC,
   team synchronization,
   compliance framework,
   and advanced auditing.
3. The page explicitly points operators to `Governance -> User Provisioning` for identity-provider configuration.
4. The page references standard virtual keys, teams, and customers as the foundation it builds on.

### Inference for next goal

1. `advanced-governance.md` is not a single page template; it is an umbrella information architecture node spanning:
   `/workspace/governance/*`,
   `/workspace/scim`,
   `/workspace/governance/rbac`,
   and `/workspace/audit-logs`.
2. The right artifact for this page is likely a family-level design spec, not a one-route page clone.
3. Public API alignment for this umbrella must be assembled from governance, users, teams, session, and any internal RBAC/SCIM contracts rather than a dedicated `advanced-governance` endpoint family.

## Audit Logs

### Observed from docs

1. Official text emphasizes immutable, cryptographically verifiable audit trails.
2. Logged domains explicitly include authentication, authorization, configuration changes, data access, and security events.
3. The page mentions SIEM export targets and alert triggers.
4. The config example uses an `audit_logs` block in `config.json`.

### Inference for next goal

1. The intended audit-log surface is semantically different from normal request logs.
2. The later implementation should not model this as just another `LLM Logs` table with minor filter differences.
3. Because the docs emphasize event class, actor, resource, verification, and compliance use cases, a future audit UI likely needs:
   event-type filtering,
   actor/resource targeting,
   integrity/verification indicators,
   and export-oriented actions.
4. Since no dedicated public `/api-reference/audit-logs/**` section exists, the next goal will need a contract decision rather than a straight public-doc port.

## Clustering

### Observed from docs

1. The page is architecture-heavy and repeatedly emphasizes peer-to-peer membership, gRPC counter sync, replicated entity types, node identity, region, and leader election.
2. Explicit fields include `node_id`, `region`, gossip port `10101`, and gRPC sync port `10102`.
3. The docs repeatedly discuss cluster status, diagnostics, liveness, and failover rather than CRUD-heavy admin objects.

### Inference for next goal

1. The intended cluster UI is probably diagnostics-first rather than configuration-form-first.
2. A faithful future page should prioritize:
   node health,
   region and leader visibility,
   drain/failover state,
   sync health,
   and replicated-state diagnostics.
3. A topology or flow visualization would be aligned with the docs emphasis, but that remains an inference because no actual dashboard screenshot was supplied.

## Custom Plugins

### Observed from docs

1. The page reads more like a service offering than a product user guide.
2. Official prose focuses on business-logic extensions, provider integrations, workflow automation, security/compliance extensions, and performance optimization.
3. No Bifrost dashboard screenshot is present.

### Inference for next goal

1. This page alone does not justify inventing a separate enterprise-only plugin management surface.
2. The safer interpretation is that the existing plugins management route remains the anchor, while enterprise-specific value may come from richer plugin types, onboarding, or support workflows not publicly documented.
3. The next goal should align this module primarily to public plugin CRUD contracts and avoid overdesigning unsupported enterprise-only UI.

## Log Exports

### Observed from docs

1. Official text describes scheduled exports, multiple destinations, multiple formats, filtering/transformation, and compliance.
2. The examples are destination config snippets for S3, GCS, Azure Blob, Snowflake, and similar sinks.
3. No Bifrost dashboard screenshot is present.

### Inference for next goal

1. The intended product surface is likely connector- and schedule-oriented rather than a simple toggle in logs settings.
2. The nearest existing OSS information architecture fit is `Observability -> Connectors`, but this is only a fit-by-shape inference.
3. The next goal should separate:
   export destinations,
   export schedule/policy,
   format/transformation config,
   and export history/status
   instead of flattening everything into a single generic settings form.

## MCP With Federated Auth

### Observed from docs

1. Official text says private enterprise APIs can be turned into MCP tools from Postman collections, OpenAPI specs, cURL commands, or a built-in UI.
2. The page emphasizes preserving request configuration and syncing user authentication into MCP tool execution.
3. Public API docs cover MCP client management and OAuth/per-user OAuth, but not this import flow as a named product surface.

### Inference for next goal

1. The future surface is likely wizard-like or import-flow-like rather than a static settings page.
2. A faithful design probably spans both MCP registry management and per-tool auth binding.
3. The built-in UI import path should not be guessed from scratch; it should be derived conservatively from the import methods named in docs and the already visible MCP registry/MCP auth surfaces.

## Overview

### Observed from docs

1. The enterprise overview positions Enterprise as a strict superset of OSS.
2. It groups capabilities into reliability/scale, governance/access control, security/compliance, and deployment options.
3. The included image is architecture-focused, not a UI screenshot.

### Inference for next goal

1. `overview.md` is a scope and taxonomy source, not a template source.
2. It is useful for deciding which surfaces belong in the final parity audit, but not for reconstructing per-page layout details.

## In-VPC Deployments

### Observed from docs

1. The page is deployment- and SLA-focused.
2. It discusses cloud providers, network isolation, compliance, uptime guarantees, and exclusions.
3. No dashboard screenshot or public API contract is present.

### Inference for next goal

1. This page should not drive a dashboard implementation unless additional evidence appears.
2. Treat it as commercial/deployment context rather than an in-product admin screen.

## Release Cadence

### Observed from docs

1. This page is commercial/support policy documentation rather than product UI documentation.

### Inference for next goal

1. No UI or API reconstruction should be derived from this page.

## Identity Provider Setup Guides

Pages:

- `setting-up-okta.md`
- `setting-up-entra.md`
- `setting-up-google-workspace.md`
- `setting-up-keycloak.md`
- `setting-up-zitadel.md`

### Observed from docs

1. All guides assume a shared Bifrost-side configuration surface and provider-specific external setup steps.
2. The shared Bifrost concepts recurring across guides are:
   redirect URI,
   client ID,
   client secret,
   optional audience/tenant/project identifiers,
   role mapping,
   team mapping,
   business-unit mapping,
   and optional provisioning credentials or tokens.
3. Google Workspace, Keycloak, and Zitadel guides explicitly add directory/provisioning credentials beyond plain login-only OIDC.
4. Okta and Entra guides repeatedly reference role and attribute mapping back into Bifrost.

### Inference for next goal

1. The future Bifrost SCIM surface should be treated as one shared shell with provider-specific field subsets, not five unrelated pages.
2. Provider-specific help content should likely be contextual and collapsible, matching the visible help accordion in the shared SCIM screenshot.
3. The next goal should not mirror the external IdP consoles visually. Those screenshots are semantic references for required fields and setup sequence only.

## Practical Boundary For The Next Goal

1. Pages without in-product screenshots should drive:
   scope decisions,
   field semantics,
   route grouping,
   and parity expectations.
2. They should not drive invented pixel-level UI unless the next goal explicitly marks the result as an inference-based template.
