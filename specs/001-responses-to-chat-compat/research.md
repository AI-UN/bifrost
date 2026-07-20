# Research: Responses To Chat Compatibility

## Decision 1: Implement the new mode in the existing compat plugin and core request dispatcher

**Decision**: Add a new compatibility flag that marks Responses requests for conversion to chat completions through the same `BifrostContextKeyChangeRequestType` mechanism already used for text-to-chat and chat-to-responses compatibility.

**Rationale**: Bifrost already has a centralized compatibility plugin, a persisted compatibility config object, and core request-dispatch branches that convert one request type into another. Reusing that path keeps the feature operator-controlled, model-catalog-aware, and symmetric with the existing compatibility behavior.

**Alternatives considered**:

- Add fallback logic to more individual providers only: rejected because it is inconsistent with the existing compatibility settings and does not give operators a single rollout switch.
- Implement the conversion only in HTTP handlers: rejected because plugin-driven request conversion is already shared by HTTP and SDK-based flows.

## Decision 2: Reuse the existing schema conversion helpers for request and response translation

**Decision**: Use `BifrostResponsesRequest.ToChatRequest()` for upstream conversion and `BifrostChatResponse.ToBifrostResponsesResponse()` plus existing streaming conversion helpers for caller-facing results.

**Rationale**: The schema layer already contains request, response, tool, and stream conversion helpers for the reverse direction. Using them avoids duplicating translation logic and keeps tool sanitization in one place.

**Alternatives considered**:

- Create a second set of conversion functions inside the compat plugin: rejected because it would duplicate behavior already present in `core/schemas/mux.go`.
- Bypass schema conversions and build raw provider payloads in the plugin: rejected because it would couple the plugin to provider-specific request shapes.

## Decision 3: Preserve model-catalog gating for when conversion is allowed

**Decision**: Only enable Responses-to-chat conversion when the model catalog indicates that Responses is unsupported and chat completions are supported for the selected provider/model combination.

**Rationale**: This matches the existing compatibility philosophy and prevents unnecessary conversions on providers with native Responses support.

**Alternatives considered**:

- Always convert when the flag is enabled: rejected because it would override native Responses support and could lose newer API semantics unnecessarily.
- Ignore provider-specific capability information and convert by model name only: rejected because Bifrost routes the same model names across different providers.

## Decision 4: Keep the feature controllable through config, UI, and `x-bf-compat`

**Decision**: Extend the existing config schema, DB-backed compat config, config hot reload path, UI toggles, and `x-bf-compat` header parsing with a new `convert_responses_to_chat` option.

**Rationale**: Operators already use those surfaces for the other compatibility modes. Adding the new flag there preserves discoverability and keeps per-request behavior consistent.

**Alternatives considered**:

- Add only a backend config field: rejected because the user explicitly requested frontend support.
- Add only a header override: rejected because global rollout control is required.
