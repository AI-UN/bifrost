# Policy And Routing Reference Spec

## Evidence Basis

- `assets/official/guardrails-overview.jpg`
- `assets/official/query-creation.jpg`
- `assets/official/cel-rule-builder.jpg`
- `assets/ui-load-balancing.png`
- `enterprise/guardrails.md`
- `enterprise/adaptive-load-balancing.md`

## Surface A: Guardrail Rules Index

### Layout Blueprint

- landing title: `Guardrail Rules`
- one-line subtitle describing when guardrails execute
- top-right green CTA: `Add New Rule`
- main body: wide bordered table
- visible columns:
  - `Rule Name`
  - `Description`
  - `Apply To`
  - `Sampling Rate`
  - `Status`

### Row Content Pattern

- rule name can show a secondary monospace or preview line below the main label
- apply target is rendered as a pill, e.g. `Both`
- enabled state uses a green switch
- row actions appear in a trailing kebab menu

## Surface B: Add/Edit Guardrail Rule Slide-Over

### Layout Blueprint

- full-height right-side drawer
- large header: `Add New Guardrail Rule`
- subtitle references CEL-based control
- sticky or clearly grouped footer actions:
  - secondary cancel
  - green save

### Required Field Order

Direct observation:

1. Rule name
2. Description
3. Enable rule toggle
4. Apply on: `Input Only`, `Output Only`, `Both`
5. Guardrail profiles multi-select
6. Sampling rate
7. Timeout
8. Rule builder
9. CEL expression preview

## Surface C: CEL Rule Builder

### Layout Blueprint

- separate builder card inside the drawer
- mode toggles: `AND` / `OR`
- builder toolbar:
  - `Add Rule`
  - `Add Rule Group`
- each rule row contains:
  - left branch connector line
  - subject dropdown, e.g. `Header`, `Model`, `Customer`
  - operator selector
  - left operand input
  - optional natural-language bridge like `has value`
  - right operand input
  - delete icon

### Expression Preview

- monospace textarea-like output
- copy button on the right
- preview updates from builder state

## Surface D: Adaptive Routing Dashboard

### Layout Blueprint

- page title focuses on live or adaptive metrics, not policy CRUD
- first section: summary metrics such as total requests and success rate
- second section: traffic-distribution table
- later sections: direction weights and route weights with performance breakdown
- each analytic table repeats provider/model filter controls

### Data Presentation

Direct observation:

- provider rows include icons
- success rate is shown prominently and in green when healthy
- tables mix weight values and penalty dimensions:
  - utilization penalty
  - error penalty
  - latency penalty
  - health status

### Interaction Model

Inference from screenshot:

- this should feel like an operator console first
- policy editing likely exists, but should not displace the metrics hierarchy on the primary screen

## API And State Notes For Later Implementation

- public contract reuse:
  - routing rules under `/api/governance/routing-rules`
- private restored contracts:
  - `/api/guardrails/**`
  - `/api/adaptive-routing/**`

## Acceptance Checks For Future Implementation

1. Guardrails open with a rules table and header CTA.
2. Rule creation uses a right-side drawer with CEL tooling.
3. Adaptive routing defaults to metrics and weighted performance views, not a CRUD-heavy form.
4. Filters and analytic tables are preserved as first-class UI elements.
