# Tasks: Responses To Chat Compatibility

**Input**: Design documents from `/specs/001-responses-to-chat-compat/`  
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/compatibility-config.md`

**Tests**: Include focused Go tests for the backend conversion path because the feature changes request routing and streaming behavior.

**Organization**: Tasks are grouped by user story to enable independent implementation and verification.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the feature artifacts and point the agent context at them.

- [X] T001 Create Speckit feature artifacts in `specs/001-responses-to-chat-compat/`
- [X] T002 Update the Speckit plan pointer in `AGENTS.md`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Extend compatibility configuration surfaces before request-path changes rely on them.

- [X] T003 Update compat config models in `framework/configstore/clientconfig.go`, `framework/configstore/tables/clientconfig.go`, and `ui/lib/types/config.ts`
- [X] T004 [P] Add `convert_responses_to_chat` to `transports/config.schema.json`
- [X] T005 [P] Extend `x-bf-compat` parsing and compat reload wiring in `transports/bifrost-http/lib/ctx.go`, `transports/bifrost-http/server/plugins.go`, and `transports/bifrost-http/handlers/config.go`
- [X] T006 Add the new compat context key and any supporting request-type metadata in `core/schemas/bifrost.go`

**Checkpoint**: Config, reload, and request-override plumbing are ready.

---

## Phase 3: User Story 1 - Responses Clients Reach Chat-Only Models (Priority: P1) 🎯 MVP

**Goal**: Convert non-streaming Responses requests to chat completions when the selected model only supports chat.

**Independent Test**: Send a non-streaming Responses request through the compat path and verify a Responses-format result is returned.

### Tests for User Story 1

- [X] T007 [P] [US1] Add compat plugin unit coverage for conversion marking in `plugins/compat/main_test.go`
- [X] T008 [P] [US1] Add schema conversion coverage for request and response fallback behavior in `core/schemas/mux_test.go`

### Implementation for User Story 1

- [X] T009 [US1] Add `convert_responses_to_chat` support to `plugins/compat/main.go`
- [X] T010 [US1] Extend non-streaming request dispatch in `core/bifrost.go` to route converted Responses requests through `ChatCompletion`
- [X] T011 [US1] Preserve caller-facing Responses envelopes and conversion metadata in `core/bifrost.go` and related schema helpers

**Checkpoint**: Non-streaming Responses fallback works end to end.

---

## Phase 4: User Story 2 - Streaming Responses Stay Compatible (Priority: P1)

**Goal**: Convert streaming Responses requests to chat streaming while still emitting Responses stream events to callers.

**Independent Test**: Send a streaming Responses request through the compat path and verify the emitted SSE events remain in Responses format.

### Tests for User Story 2

- [X] T012 [P] [US2] Add stream conversion coverage in `core/schemas/mux_test.go` or a dedicated core test file
- [X] T013 [P] [US2] Add request-dispatch coverage for Responses stream fallback in `core/bifrost_test.go` or the nearest existing test target

### Implementation for User Story 2

- [X] T014 [US2] Extend streaming request dispatch in `core/bifrost.go` for converted Responses stream requests
- [X] T015 [US2] Update stream post-hook conversion handling in `core/utils.go` if needed so post-hooks see caller-facing Responses stream chunks
- [X] T016 [US2] Ensure converted stream requests set the chat-fallback context needed by OpenAI-compatible stream handlers

**Checkpoint**: Streaming Responses fallback works end to end.

---

## Phase 5: User Story 3 - Operators Control Compatibility Centrally And Per Request (Priority: P2)

**Goal**: Expose and persist the new compatibility mode in the UI and operator config flows.

**Independent Test**: Enable the new toggle in UI/config, save, reload config, and verify per-request override behavior with `x-bf-compat`.

### Implementation for User Story 3

- [X] T017 [US3] Add the new toggle to `ui/app/workspace/config/views/compatibilityView.tsx`
- [X] T018 [US3] Update compatibility docs in `docs/features/compat-plugin.mdx`

**Checkpoint**: Operators can discover, enable, and understand the feature.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final verification and cleanup across stories.

- [X] T019 [P] Run targeted backend tests for `core/schemas`, `plugins/compat`, and affected transport/config packages
- [X] T020 Review generated artifacts and implementation for consistency with `specs/001-responses-to-chat-compat/spec.md`
