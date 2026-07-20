# Governance And Identity Reference Spec

## Evidence Basis

- `assets/rbac-list.png`
- `assets/rbac-edit-role.png`
- `assets/official/scim-overview.jpg`
- `assets/official/scim-attribute-mapping.jpg`
- `assets/official/scim-import-preview.jpg`
- `enterprise/rbac.md`
- `enterprise/user-provisioning.md`
- provider setup guides under `enterprise/setting-up-*.md`

## Shell Pattern

Direct observation:

- large white rounded shell over a soft blue/pink gradient background
- persistent left navigation with section accordions
- selected leaf items use green emphasis
- main work area is visually quiet and left-aligned, with very wide forms/tables

Implementation constraint for later coding AI:

- do not collapse these screens into generic stacked cards
- preserve the official two-region composition when evidence supports it:
  - selector rail or list on the left
  - detailed editor/form on the right

## Surface A: Roles & Permissions

### Layout Blueprint

- page title: `Roles & Permissions`
- supporting line: short sentence describing role/permission management
- primary CTA in top-right: green `Add Role`
- main body: wide table
- visible columns from screenshot:
  - `Name`
  - `Description`
  - `Type`
  - `Permissions`
- row actions:
  - kebab menu on the far right

### Detail/Edit Pattern

- official editor evidence is a right-side slide-over, not an inline accordion
- the edit surface should own:
  - role metadata
  - permission assignment groups
  - save/cancel actions

### Data Model Expectations

Direct fact:

- roles have a type badge such as `System`
- roles carry a visible permission count

Inference:

- system roles and custom roles likely share the same listing surface
- edit drawer should preserve permission grouping by resource family

## Surface B: User Provisioning Provider Config

### Layout Blueprint

- left middle pane: provider list with one active selection at a time
- right pane: provider-specific configuration form
- provider rail includes logos and active status badges
- selected provider row uses a soft selected background
- right pane begins with:
  - large title
  - one-line subtitle
  - collapsible help card / setup hint

### Provider Form Structure

Direct observation from Okta/Entra screenshots:

- long vertical form with generous spacing
- labeled text fields
- optional and secret fields are explicitly marked
- helper text appears below many fields
- eye icons appear for secret visibility toggles

### Required Reusable Sections

- provider credentials
- issuer / audience / endpoint fields
- API token or secret fields
- mapping sections for roles, teams, and business units

## Surface C: Attribute Mapping Composer

### Layout Blueprint

- mapping sections are rendered as separate bordered cards
- each mapping card contains:
  - title
  - explanatory copy
  - blue informational banner listing required IdP attributes
  - repeatable mapping rows
  - `Add Mapping` secondary CTA

### Mapping Row Pattern

- drag handle on the far left
- source claim dropdown
- value input
- target Bifrost entity selector or derived display
- delete icon on the far right

### Semantic Rules

Direct observation:

- role mapping resolves to the highest-permission match
- team mapping can fan out to multiple teams
- wildcard `*` can sync all values as team names directly

Inference:

- business-unit mapping should enforce single resolved BU semantics

## Surface D: Sync Users Modal

### Layout Blueprint

- launched on top of the users table as a centered modal
- dimmed page behind modal
- title example: `Sync Users from IdP`
- step-style modal that starts with filter selection
- bottom-right action cluster:
  - secondary `Cancel`
  - green primary `Next`

### Filter Control Pattern

- grouped checkbox list inside a bordered panel
- `Sync from all` appears as the first aggregate option
- provider-specific groups appear below

## API And State Notes For Later Implementation

- public substrate that is safe to reuse directly:
  - session
  - OAuth / per-user OAuth
  - baseline users/teams concepts
- private restored contracts that require deliberate reconciliation:
  - `/api/rbac/**`
  - `/api/scim/auth-type`
  - `/api/sso/**`
  - `/api/user-groups/**`
  - `/api/access-profiles`

## Acceptance Checks For Future Implementation

1. RBAC uses a role table plus right-side editor, not a one-page admin workbench.
2. SCIM uses provider selector plus detailed form, not a simple provider CRUD table.
3. Attribute mapping cards exist for roles, teams, and business units.
4. User sync runs through a modal or wizard step consistent with the official screenshots.
