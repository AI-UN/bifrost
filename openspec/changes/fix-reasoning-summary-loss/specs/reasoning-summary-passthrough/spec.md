## ADDED Requirements

### Requirement: OpenAI Responses API raw passthrough SHALL be enabled by default
When the target provider is OpenAI and the request is for the Responses API, the integration layer SHALL forward original OpenAI SSE events to the client without adding extraneous fields.

#### Scenario: OpenAI Responses API streaming with default config
- **WHEN** a client sends a Responses API streaming request to Bifrost targeting the OpenAI provider
- **THEN** the integration layer SHALL return the original OpenAI SSE event JSON to the client (raw passthrough), without adding `sequence_number`, `extra_fields`, or default values via `WithDefaults()`

#### Scenario: Non-OpenAI provider falls back to WithDefaults
- **WHEN** a client sends a Responses API streaming request targeting a non-OpenAI provider
- **THEN** the integration layer SHALL use `WithDefaults()` to normalize the response (no raw passthrough available)

#### Scenario: Raw passthrough does not break post-hooks
- **WHEN** raw passthrough is enabled for OpenAI Responses API
- **THEN** the accumulator SHALL still process events internally for post-hooks, logging, and tracing

### Requirement: Reasoning summary part added events SHALL be accumulated
The streaming accumulator SHALL handle `response.reasoning_summary_part.added` events by creating or locating the corresponding reasoning message structure in the accumulated response.

#### Scenario: Reasoning summary part added for new reasoning item
- **WHEN** a `response.reasoning_summary_part.added` event arrives with an `item_id` that has no existing reasoning message
- **THEN** the accumulator SHALL create a new `ResponsesMessage` with `Type: "reasoning"` and an empty `ResponsesReasoning.Summary` array, indexed by `item_id`

#### Scenario: Reasoning summary part added for existing reasoning item
- **WHEN** a `response.reasoning_summary_part.added` event arrives with an `item_id` that already has a reasoning message
- **THEN** the accumulator SHALL append a new `ResponsesReasoningSummary` entry with `Type: "summary_text"` and empty `Text`

### Requirement: Reasoning summary text delta events SHALL append to summary
The streaming accumulator SHALL handle `response.reasoning_summary_text.delta` events by appending the delta text to the reasoning summary.

#### Scenario: Delta without ContentIndex appends to ResponsesReasoning.Summary
- **WHEN** a `response.reasoning_summary_text.delta` event arrives without a `content_index`
- **THEN** the accumulator SHALL append `delta` to `ResponsesReasoning.Summary[last].Text` on the matching reasoning message

#### Scenario: Delta with ContentIndex appends to content block
- **WHEN** a `response.reasoning_summary_text.delta` event arrives with a `content_index`
- **THEN** the accumulator SHALL append `delta` to `Content.ContentBlocks[content_index].Text` on the matching reasoning message

#### Scenario: Signature in delta SHALL be preserved
- **WHEN** a `response.reasoning_summary_text.delta` event includes a `signature` field
- **THEN** the accumulator SHALL append the signature to `ResponsesReasoning.EncryptedContent` (for summary path) or `Content.ContentBlocks[content_index].Signature` (for content block path)

### Requirement: Reasoning summary text done events SHALL finalize the summary part
The streaming accumulator SHALL handle `response.reasoning_summary_text.done` events by marking the current summary text as complete.

#### Scenario: Done event finalizes summary text
- **WHEN** a `response.reasoning_summary_text.done` event arrives for an `item_id`
- **THEN** the accumulator SHALL ensure the accumulated summary text for that part is complete and any metadata from the done event is preserved

### Requirement: Reasoning summary part done events SHALL be handled
The streaming accumulator SHALL handle `response.reasoning_summary_part.done` events without losing data.

#### Scenario: Part done finalizes the reasoning summary part
- **WHEN** a `response.reasoning_summary_part.done` event arrives
- **THEN** the accumulator SHALL mark the corresponding summary part as complete

### Requirement: Reasoning output items SHALL be preserved in accumulated output
The accumulator SHALL include reasoning-type output items (`item.type: "reasoning"`) in the accumulated `Output` array.

#### Scenario: Output item added with reasoning type
- **WHEN** a `response.output_item.added` event arrives with `item.type: "reasoning"`
- **THEN** the accumulator SHALL append the item to the `Output` array with its `ResponsesReasoning` data intact

#### Scenario: Output item done with reasoning type
- **WHEN** a `response.output_item.done` event arrives with `item.type: "reasoning"`
- **THEN** the accumulator SHALL update the corresponding output item with final reasoning data including `summary` and `encrypted_content`

#### Scenario: Encrypted content extracted only from output_item.done
- **WHEN** a `response.output_item.done` event arrives with `item.type: "reasoning"` containing `encrypted_content` in the reasoning data
- **THEN** the accumulator SHALL store `encrypted_content` in `ResponsesReasoning.EncryptedContent`
- **AND** summary text done events (`response.reasoning_summary_text.done`) SHALL NOT be expected to carry `encrypted_content` (they carry `signature` only)

### Requirement: Reasoning events SHALL pass through the integration layer
The HTTP integration layer SHALL forward all reasoning-related fields from `BifrostResponsesStreamResponse` to the client without stripping or transforming them.

#### Scenario: Stream response with reasoning delta
- **WHEN** the integration layer receives a `BifrostResponsesStreamResponse` with type `response.reasoning_summary_text.delta`
- **THEN** the layer SHALL serialize and forward the `Delta` field (and `Signature` if present) to the client unchanged

#### Scenario: Non-streaming response with reasoning output
- **WHEN** the integration layer receives a `BifrostResponsesResponse` with reasoning items in `Output`
- **THEN** the layer SHALL include the reasoning items with `summary`, `encrypted_content`, and all nested fields in the serialized response
