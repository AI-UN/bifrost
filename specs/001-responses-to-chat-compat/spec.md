# Feature Specification: Responses To Chat Compatibility

**Feature Branch**: `[001-responses-to-chat-compat]`  
**Created**: 2026-05-06  
**Status**: Implemented  
**Input**: User description: "Implement a new Bifrost compatibility option that converts OpenAI Responses API requests into Chat Completions requests when the selected upstream model does not support Responses natively, including backend and frontend support."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Responses Clients Reach Chat-Only Models (Priority: P1)

An operator enables the new compatibility option so applications that only speak the OpenAI Responses API can still use providers and models that only support chat completions upstream.

**Why this priority**: This is the core business value of the feature. Without it, newer Responses-based clients cannot access older or chat-only upstream models through Bifrost.

**Independent Test**: Can be fully tested by sending a non-streaming Responses API request to a model that does not support Responses but does support chat completions, and verifying the caller receives a valid Responses-format result instead of an unsupported-operation failure.

**Acceptance Scenarios**:

1. **Given** the compatibility option is enabled and the selected model supports chat completions but not Responses, **When** a caller submits a Responses request, **Then** Bifrost routes an equivalent chat completion upstream and returns a Responses-format result to the caller.
2. **Given** the compatibility option is disabled, **When** the same caller submits the same Responses request, **Then** Bifrost keeps the existing native behavior and does not apply the fallback conversion.

---

### User Story 2 - Streaming Responses Stay Compatible (Priority: P1)

An operator enables the same compatibility option for streaming workloads so Responses API streaming clients can consume server-sent events even when the upstream model only supports chat completion streaming.

**Why this priority**: Streaming is a first-class path in Bifrost. A compatibility layer that only works for non-streaming requests would be incomplete and would still block common client integrations.

**Independent Test**: Can be fully tested by sending a streaming Responses request to a chat-only model and verifying that the caller receives a valid Responses stream shape from start to completion.

**Acceptance Scenarios**:

1. **Given** the compatibility option is enabled and the selected model only supports chat streaming upstream, **When** a caller sends a streaming Responses request, **Then** Bifrost converts the upstream request to chat streaming and emits Responses-format stream events back to the caller.
2. **Given** the upstream chat stream finishes successfully, **When** Bifrost finishes the fallback flow, **Then** the caller receives a terminal Responses completion event with preserved usage and metadata when available.

---

### User Story 3 - Operators Control Compatibility Centrally And Per Request (Priority: P2)

An operator can configure the new compatibility mode in Bifrost configuration and the UI, while advanced clients can still enable it on a single request through the compatibility header.

**Why this priority**: The fallback must be operable in real deployments, not only hardcoded in backend logic. Operators need visibility and control over rollout.

**Independent Test**: Can be fully tested by enabling the option in configuration or UI, verifying persistence and hot reload, and then overriding it for a single request through the compatibility header.

**Acceptance Scenarios**:

1. **Given** an operator enables the new option in the Compatibility settings page or config payload, **When** the configuration is saved, **Then** the new value persists and is used for subsequent requests without requiring callers to change request payloads.
2. **Given** the global option is disabled, **When** a caller sends a request with a per-request compatibility override that includes the new mode, **Then** Bifrost applies the conversion only for that request.

### Edge Cases

- What happens when a model supports neither Responses nor chat completions? The request should continue to fail normally without a misleading fallback.
- How does the system handle Responses-only request fields or tool definitions that cannot be represented safely in chat-completions format? The fallback should degrade predictably using existing sanitization rules rather than sending malformed upstream payloads.
- What happens when the model catalog is missing capability data? The compatibility mode should not force a conversion unless Bifrost can determine that chat is supported and Responses is not.
- How does the system handle streaming or non-streaming failures after conversion starts? Error metadata should still reflect the caller-facing request type and the fact that a conversion was attempted.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a new compatibility setting that enables Responses-to-chat request conversion when an upstream model supports chat completions but does not support the Responses API.
- **FR-002**: System MUST persist the new setting in the same client compatibility configuration used for the existing compatibility options.
- **FR-003**: System MUST expose the new setting in the configuration UI alongside the existing compatibility toggles.
- **FR-004**: System MUST allow the new setting to be enabled per request through the existing compatibility override mechanism.
- **FR-005**: System MUST only apply the conversion when the selected model lacks Responses support and has chat-completions support.
- **FR-006**: System MUST convert non-streaming Responses requests into equivalent chat-completions requests upstream and return a caller-facing Responses result.
- **FR-007**: System MUST convert streaming Responses requests into equivalent chat-completions streaming requests upstream and return a caller-facing Responses event stream.
- **FR-008**: System MUST preserve the original routing inputs, including provider, model, and fallback chain, when the conversion is applied.
- **FR-009**: System MUST preserve caller-facing metadata that indicates a compatibility conversion occurred.
- **FR-010**: System MUST preserve existing behavior for requests that do not require the conversion or when the compatibility option is disabled.
- **FR-011**: System MUST continue to use safe fallback behavior for request fields that cannot be represented directly in chat-completions format.
- **FR-012**: System MUST allow the compatibility plugin to be loaded, reloaded, and removed using the new setting without breaking the existing compatibility options.

### Key Entities *(include if feature involves data)*

- **Compatibility Setting**: The persisted operator-controlled flag that enables Responses-to-chat conversion globally.
- **Request Conversion Decision**: The per-request determination of whether the selected model should stay on Responses or be converted to chat completions.
- **Caller-Facing Response Envelope**: The response or stream format returned to the client, which must remain in Responses format even when chat completions are used upstream.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An operator can enable the new compatibility option through configuration or UI, save it successfully, and see the same value returned on the next configuration read.
- **SC-002**: A non-streaming Responses request sent to a chat-only model succeeds through the compatibility path and returns a caller-facing Responses result without requiring client payload changes.
- **SC-003**: A streaming Responses request sent to a chat-only model completes through the compatibility path and emits a valid Responses-format event sequence from the first delta through the terminal event.
- **SC-004**: When the option is disabled, existing request handling for native Responses requests remains unchanged.

## Assumptions

- The existing model catalog remains the source of truth for whether a model supports Responses and chat-completions request types.
- Existing request and response conversion helpers in the core schemas are acceptable building blocks for the new compatibility path.
- Existing per-provider Responses fallbacks remain valid and should continue to work; the new compatibility mode adds centralized operator control rather than replacing all provider-specific logic.
- The compatibility settings page is the correct place for the new operator-facing toggle.
