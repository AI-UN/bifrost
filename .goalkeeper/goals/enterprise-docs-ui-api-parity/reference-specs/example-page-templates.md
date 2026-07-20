# Example Page Templates For Non-Multimodal Implementation

## How To Use This File

These templates are not production code. They are structured layout contracts that a later coding AI can translate into real Bifrost pages while staying aligned to the official evidence.

Each template separates:

- `direct`: visible in official screenshots or explicit in docs
- `inferred`: reconstruction required because the official evidence is partial

## Template 1: SCIM Provider Configuration

### Direct

- left provider rail with selectable IdP rows
- large page title and one-line description on the right
- collapsible setup/help banner near the top
- long vertically stacked provider form
- mapping cards for roles, teams, and business units

### Inferred Wireframe

```text
Workspace shell
  Left nav
  Main shell
    Provider rail
      Search or static list
      Provider row
      Provider row (active)
    Detail pane
      Title
      Subtitle
      Help accordion
      Credentials section
      Endpoint section
      Secret section
      Role mapping card
      Team mapping card
      Business unit mapping card
      Footer actions
```

## Template 2: Guardrail Rule Editor

### Direct

- table landing page
- green `Add New Rule` CTA
- slide-over rule editor
- apply-direction choice chips
- guardrail profile selector
- sampling and timeout fields
- CEL builder with AND/OR toolbar
- CEL expression preview with copy action

### Inferred Wireframe

```text
Rules page
  Header: title + subtitle + primary CTA
  Rules table
    columns: name, description, apply_to, sampling_rate, status, actions

Slide-over editor
  Header
  Rule metadata section
  Enable toggle
  Apply-on segmented controls
  Profiles multi-select
  Numeric settings row
  CEL builder card
  CEL preview card
  Footer: cancel + save
```

## Template 3: Adaptive Routing Dashboard

### Direct

- metrics headline cards
- traffic distribution table
- weighted performance tables
- repeated provider/model filters
- provider icons and health/status emphasis

### Inferred Wireframe

```text
Adaptive Routing
  Header
  Summary strip
    Total requests
    Success rate
    Live connection state
  Section: Traffic distribution
    Filters
    Table
  Section: Direction weights and performance
    Filters
    Table
  Section: Route weights and performance
    Filters
    Table
```

## Template 4: Datadog Connector

### Direct

- provider selector rail inside the page
- Datadog-specific form
- LLM Observability toggle
- connection mode selector
- environment/version row
- repeatable custom tags table

### Inferred Wireframe

```text
Observability / Connectors
  Provider rail
    OpenTelemetry
    Maxim
    Datadog (active)
    New Relic (coming soon)
  Detail pane
    Title + subtitle
    Service name
    LLM Observability toggle row
    ML app name
    Connection mode
    Endpoint/address
    Environment + version
    Custom tags table
    Footer actions
```

## Template 5: Users Sync Modal

### Direct

- modal on top of users list
- filter by groups
- checkbox list
- cancel/next actions

### Inferred Wireframe

```text
Sync Users From IdP
  Intro copy
  Filter group card
    Sync all checkbox
    Provider group options
  Footer
    Cancel
    Next
```
