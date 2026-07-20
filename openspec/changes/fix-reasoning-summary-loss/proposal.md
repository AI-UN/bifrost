## Why

Reasoning summaries are silently dropped when clients (OpenCode, Claude Code, Codex) route Responses API requests through Bifrost. The same clients work correctly when connected directly to CLIProxyAPI or OpenAI. Chat Completions models (e.g., DeepSeek) are unaffected because reasoning text is embedded in the `content` field.

Root cause: Bifrost's `WithDefaults()` method adds extraneous fields (`sequence_number`, `extra_fields`, default `logprobs: []`) to every stream event when raw passthrough is not enabled (the default). OpenCode's `@ai-sdk/openai` SDK expects exact OpenAI wire format and may reject or misparse these augmented events. Additionally, Bifrost's streaming accumulator is missing handlers for 5 of 6 reasoning event types, causing incomplete accumulated data for post-hooks and non-OpenAI providers.

## What Changes

- Enable raw passthrough by default for OpenAI provider Responses API requests, so original OpenAI SSE events reach clients unchanged (primary fix)
- Add missing reasoning event handlers in Bifrost's Responses streaming accumulator (`framework/streaming/responses.go`): `reasoning_summary_part.added`, `reasoning_summary_text.done`, `reasoning_summary_part.done` (secondary fix for non-OpenAI providers and post-hooks)
- Fix dual-path reasoning accumulation that routes deltas to different structures based on `ContentIndex`
- Ensure `encrypted_content` is correctly extracted from `output_item.done` events (not from summary text events — it's only in `output_item.done`)

## Capabilities

### New Capabilities

- `reasoning-summary-passthrough`: Ensure Bifrost's Responses API streaming path preserves all reasoning summary events (`reasoning_summary_part.added`, `reasoning_summary_text.delta`, `reasoning_summary_text.done`, `reasoning_summary_part.done`) and reasoning output items (`output_item.added/done` with type `reasoning`) without loss or transformation.

### Modified Capabilities

(none — this is a bug fix, not a requirements change)

## Impact

- `framework/streaming/responses.go` — streaming accumulator (primary fix)
- `core/schemas/responses.go` — may need schema additions for missing stream event fields
- `core/providers/openai/responses.go` — verify upstream events are passed through correctly
- `transports/bifrost-http/integrations/openai.go` — verify integration layer preserves reasoning fields
- No breaking changes — this adds missing event handling, does not change existing behavior
