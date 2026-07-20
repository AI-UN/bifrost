# Data Model: Responses To Chat Compatibility

## Compatibility Setting

- **Purpose**: Stores whether centralized Responses-to-chat conversion is enabled for the deployment.
- **Fields**:
  - `convert_responses_to_chat`: boolean flag
- **Relationships**:
  - Lives inside the existing `client_config.compat` object beside the other compatibility flags.
- **Validation rules**:
  - Defaults to `false`
  - Must round-trip through config schema, persisted config store, API payloads, and UI state

## Request Conversion Decision

- **Purpose**: Represents the per-request decision to convert a Responses request into a chat-completions request.
- **Fields**:
  - `original_request_type`
  - `target_request_type`
  - `provider`
  - `model`
  - `enabled_by` (global config or request override)
- **Relationships**:
  - Derived from the request plus model-catalog capability checks
  - Stored transiently in request context
- **Validation rules**:
  - Only valid when original type is `responses` or `responses_stream`
  - Only valid when target type is `chat_completion`

## Caller-Facing Response Envelope

- **Purpose**: Ensures the client continues to receive Responses-format output even when the upstream request was converted to chat-completions format.
- **Fields**:
  - `request_type`
  - `converted_request_type`
  - `usage`
  - `output` or streaming events
- **Relationships**:
  - Built from a converted upstream chat response or chat stream
  - Carries compatibility metadata back through existing Bifrost response extra fields
- **Validation rules**:
  - Caller-facing type must remain Responses
  - Conversion metadata must remain attached after plugin post-hooks
