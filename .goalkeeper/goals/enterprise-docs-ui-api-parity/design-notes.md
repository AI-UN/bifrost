# Screenshot-Backed Design Notes

## How To Read This File

- `Observed` means the detail is directly visible in a supplied screenshot.
- `Inference` means the detail is a best-effort interpretation that is not fully visible in the screenshot itself.
- This file covers only screenshot-backed surfaces for `GK-003`.
- Provider setup guides such as `setting-up-okta.md` or `setting-up-entra.md` include many external identity-provider console screenshots. Those are evidence for setup flow and field semantics, but they are not treated here as Bifrost dashboard layout templates.

## Cross-Surface Visual Language

### Observed

1. All supplied enterprise screenshots place the product inside a large white, rounded-corner workspace shell over a soft gradient background.
2. The shell uses a persistent left sidebar and a wide right-hand content area.
3. Active navigation items are highlighted with a soft green-tinted background and dark green icon/text treatment.
4. Primary action buttons are green, rectangular with rounded corners, and usually live at the top-right of the content area.
5. Table surfaces use very light borders, tall rows, and strong black headings.
6. Forms use stacked labeled inputs with helper text directly below each field.
7. Secondary structural panels often use a pale off-white background distinct from the pure white main canvas.
8. Slide-over or modal forms darken the main page instead of replacing it.

### Observed Version Drift

1. At least two closely related UI generations appear in the screenshots.
2. The RBAC screenshots show an earlier shell variant: no visible global search input in the sidebar, and a slightly broader governance-heavy menu.
3. The SCIM, Guardrails, and Datadog screenshots show a newer shell variant: a top sidebar search field, more compact typography, and more card-like internal subpanels.
4. The right implementation target should preserve structural patterns that repeat across both generations rather than overfitting to one screenshot's exact spacing.

## Global Navigation

### Observed

1. Top-level left-nav items repeatedly visible across screenshots include `Observability`, `Prompt Repository`, `Model Providers`, `MCP Gateway`, `Plugins`, `Governance`, `Guardrails`, `Cluster Config`, `Adaptive Routing`, and `Config`.
2. `Plugins` is marked with a `BETA` badge in multiple screenshots.
3. `Evals` appears as an external-link style item in newer screenshots.
4. `Observability` contains at least `Dashboard`, `LLM Logs`, `MCP Logs`, `Connectors`, and `Logs Settings`.
5. `Governance` contains at least `Virtual Keys`, `Users`, `Teams`, `Business Units`, `Customers`, `User Provisioning`, `Roles & Permissions`, `Access Profiles`, and `Audit Logs`.
6. `Guardrails` contains at least `Configuration` and `Providers`.
7. The sidebar footer shows icon-only controls and a version label such as `v1.2.3-enterprise`.

### Inference

1. The navigation hierarchy is not purely route-based; some entries likely switch tabs or subviews within a broader module shell.

## RBAC: Roles List

Source:

- `assets/rbac-list.png`

### Observed

1. The main page title is `Roles & Permissions` with a one-line subtitle directly underneath.
2. A single primary button `Add Role` sits in the top-right.
3. The primary content is a wide table card.
4. Table columns visible are `Name`, `Description`, `Type`, and `Permissions`, followed by a trailing actions column.
5. System roles appear as rows with a pill-like `System` tag in the `Type` column.
6. Permissions are shown as compact numeric counts such as `42`, `27`, and `14`, not expanded badges.
7. The table rows are tall and sparse, emphasizing readability over density.

### Inference

1. The trailing `...` action likely opens edit/manage/delete operations, but only the existence of the menu trigger is directly visible.

## RBAC: Manage Permissions Dialog

Source:

- `assets/rbac-edit-role.png`

### Observed

1. Permission management opens in a large right-side slide-over sheet that covers most of the viewport height and roughly half of the viewport width.
2. The sheet title is role-specific, for example `Manage Permissions for Admin`.
3. The internal layout is two-column.
4. The left column is a vertically scrollable `Resources` list.
5. Each resource row shows a name and a compact enabled-count summary such as `4/4 permissions`.
6. The selected resource row uses a green outline/accent state.
7. The right column shows the selected resource name plus a list of operation cards or rows.
8. Each permission row contains an operation name, a short description, and a right-aligned toggle.
9. The sheet footer contains a total-selected summary and a green primary save action.

### Inference

1. The permission rows probably repeat `View`, `Create`, `Update`, and `Delete` where applicable, but the screenshot only shows the selected `Logs` resource with `View`.

## User Provisioning / SCIM: Shared Shell

Source:

- `assets/official/scim-overview.jpg`
- `assets/official/scim-provider-select.jpg`

### Observed

1. The SCIM page uses a three-part horizontal composition:
   left global sidebar, middle provider selector panel, right configuration form.
2. The middle provider panel is titled `Providers`.
3. Providers are listed as large vertical rows with icons and generous spacing.
4. The selected provider row uses a pale highlighted card treatment.
5. An `ACTIVE` badge can appear inline beside a provider entry.
6. The right panel title changes with the selected provider, for example `SCIM Configuration`.
7. The right panel subtitle is provider-specific, for example `Configure your Okta SCIM provider settings`.
8. A pale callout/help accordion appears near the top of the form.
9. Long provider configuration forms are composed of repeated label, input, helper-text stacks.
10. Sensitive fields such as secrets or tokens show an eye icon at the right edge of the input.
11. Mapping sections appear as their own boxed cards inside the form, each with a heading, descriptive copy, and an `Add Mapping` button.
12. The bottom action area includes at least `Verify Configuration`, `Reset`, and `Save Configuration`.

### Inference

1. The action area is likely sticky or visually anchored near the bottom because it remains visible in a very long form screenshot.

## User Provisioning / SCIM: Mapping Sections

Source:

- `assets/official/scim-overview.jpg`

### Observed

1. Separate mapping cards exist for `Attribute-to-Role Mappings`, `Attribute-to-Team Mappings`, and `Attribute-to-Business Unit Mappings`.
2. Each mapping card includes explanatory copy about matching behavior.
3. Team and business-unit mapping cards include blue informational callouts.
4. The page favors explicit explanatory text over terse enterprise jargon.

## Guardrails: Rules Index

Source:

- `assets/official/guardrails-overview.jpg`

### Observed

1. The visible page heading is `Guardrail Rules`.
2. The page subtitle explains the screen purpose in plain language.
3. A top-right primary action `Add New Rule` creates new rules.
4. The main content is a table card.
5. Visible columns include `Rule Name`, `Description`, `Apply To`, `Sampling Rate`, `Status`, plus trailing actions.
6. Rule name cells can include a smaller secondary line beneath the main name.
7. `Apply To` is represented with a compact pill such as `Both`.
8. `Status` is represented with a toggle, not a text-only badge.

## Guardrails: Rule Creation Sheet

Source:

- `assets/official/query-creation.jpg`

### Observed

1. Rule creation opens in a large right-side slide-over sheet similar to the RBAC permission editor.
2. The title is `Add New Guardrail Rule`.
3. The top of the form contains `Rule Name` and `Description`.
4. Enablement is represented as a large inline toggle section rather than a small switch beside the title.
5. `Apply on` is presented as radio-card choices: `Input Only`, `Output Only`, `Both`.
6. `Guardrail Profiles` uses a chip-like multi-select surface.
7. `Sampling Rate (%)` and `Timeout (seconds)` are plain numeric inputs.
8. The `Rule Builder` section is a large embedded builder surface.
9. The footer uses `Cancel` and green `Save Rule` actions.

## Guardrails: Rule Builder

Source:

- `assets/official/cel-rule-builder.jpg`

### Observed

1. The builder is visually boxed and starts with logic controls: `AND`, `OR`, `Add Rule`, `Add Rule Group`.
2. Rule rows are composed from left-to-right field chips/dropdowns and value inputs.
3. Visible operand types include `Header`, `Model`, and `Customer`.
4. A generated `CEL Expression Preview` appears beneath the visual builder.
5. The preview surface includes a `Copy` action.
6. The builder explicitly translates visual rules into a final CEL expression string.

## Guardrails: Provider Configurations

Source:

- `assets/official/provider-aws-create.jpg`

### Observed

1. The providers screen also uses the middle selector / right detail pattern.
2. The provider selector includes at least `AWS Bedrock`, `Azure Content Moderation`, `Patronus AI`, `Mistral moderation`, and `Pangea`.
3. `Pangea` is shown with a `COMING SOON` badge.
4. The right panel title is provider-specific, for example `Bedrock Guardrail Configurations`.
5. Existing provider configs are shown in a table with columns like `ID`, `Name`, `Is Enabled`, and `Timeout (s)`.
6. A top-right action creates a new provider configuration.

### Inference

1. The provider selector is likely shared between rules and providers subroutes, but the screenshot only proves that the same visual language is reused.

## Datadog Connector

Source:

- `assets/official/dd-config-page.jpg`

### Observed

1. Datadog lives under `Observability -> Connectors`.
2. The page uses the same middle selector / right detail composition as SCIM and Guardrails providers.
3. The selector lists at least `Open Telemetry`, `Maxim`, `Datadog`, and `New Relic`.
4. `New Relic` is visibly disabled or unavailable and marked `COMING SOON`.
5. The Datadog right-hand form begins with a descriptive sentence instead of a large heading.
6. The form includes:
   `Service Name`,
   `LLM Observability` toggle,
   `ML App Name`,
   `Connection Mode`,
   `Agent Address`,
   `Environment`,
   `Version`,
   and a `Custom Tags` table-like editor.
7. The custom-tags area uses row-based name/value inputs with per-row delete icons.

### Inference

1. The missing lower portion likely contains save/reset controls similar to SCIM, but they are not visible in the supplied screenshot.

## Adaptive Load Balancing / Adaptive Routing

Source:

- `assets/ui-load-balancing.png`

### Observed

1. The selected nav item is `Adaptive Routing`.
2. The page title is `Live Metrics`.
3. The hero area surfaces large KPI numerics such as `Total Requests` and `Success Rate`.
4. A compact live-status strip appears near the top-right with connection state and a clock/time value.
5. The first table section is `Total Traffic Distribution in the last 10s`.
6. The first table includes `Key`, `Provider`, `Model`, and `Total Traffic`.
7. Traffic rows show inline horizontal bars rather than plain numerics alone.
8. Filter dropdowns exist per table block, including `All Providers` and `All Models`.
9. The next visible section is `Direction Weights & Performance`.
10. Direction rows include provider, model, weight, success rate, errors, utilization penalty, error penalty, latency penalty, and health status.
11. A further `Route Weights & Performance` table is visible lower on the page.
12. Health states use small green pill badges such as `healthy`.

### Inference

1. The page likely has no primary CRUD button because it is metrics-first, but the screenshot does not prove whether a policy editor exists elsewhere on the route.

## Design Implications For The Next Goal

These are handoff implications, not implementation instructions.

1. The repeated enterprise pattern is not a generic card grid. It is a deliberate operator console with deep vertical forms, large tables, and clear left-to-right task decomposition.
2. For configuration-heavy modules, the dominant pattern is:
   provider or resource selector on the left,
   long details form or table on the right.
3. For rules-heavy modules, the dominant pattern is:
   wide index table,
   large right-side slide-over for creation/editing,
   strong explanatory helper text,
   explicit preview or summary region.
4. The next implementation goal should preserve the operator-console density and long-form ergonomics visible here rather than simplifying these surfaces into marketing-style placeholders or shallow settings cards.
