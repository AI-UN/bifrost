# Connectors And Observability Reference Spec

## Evidence Basis

- `assets/official/dd-config-page.jpg`
- `assets/official/dd-mode.jpg`
- `assets/official/dd-llmobs.jpg`
- `assets/official/dd-trace.jpg`
- `enterprise/datadog-connector.md`

## Surface: Datadog Connector

### Layout Blueprint

- left inner rail titled `Providers`
- selectable connector rows with logos
- current examples visible in screenshots:
  - Open Telemetry
  - Maxim
  - Datadog
  - New Relic (`COMING SOON`)
- right pane hosts the Datadog-specific form

### Datadog Form Structure

Direct observation:

1. service name
2. LLM Observability toggle
3. ML app name
4. connection mode selector
5. agent or transport address field
6. environment and version fields in a two-column row
7. custom tags repeatable table

### Control Language

- labels are plain and operational, not marketing-oriented
- helper copy sits under most labels
- toggles are right-aligned within the field row
- repeatable tags use a two-column mini-table with delete affordances

## State Notes

Direct observation:

- the page is a dedicated configuration surface, not merely a generic connector shell
- connection mode is a first-class concept
- LLM Observability is prominent enough to sit near the top of the form

Inference:

- save/delete/test actions likely exist lower on the page or in an unseen footer region

## API Notes For Later Implementation

- no dedicated public Datadog API contract was found
- restored branch uses private generic connector routes:
  - `/api/connectors`
  - `/api/connectors/{id}`
  - `/api/connectors/{id}/test`

## Acceptance Checks For Future Implementation

1. Datadog gets a dedicated form layout and copy hierarchy.
2. The left provider selector remains visible.
3. Connection mode, LLM Observability, and tag rows are first-class controls.
4. The surface should not read like a generic key/value connector editor.
