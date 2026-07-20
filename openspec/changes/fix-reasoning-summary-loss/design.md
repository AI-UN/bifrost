## Context

Bifrost is a high-performance AI gateway that proxies requests between clients and LLM providers. When clients send OpenAI Responses API requests through Bifrost, the gateway must faithfully forward all streaming events, including reasoning summary events.

The Responses API defines several reasoning-related SSE events:
- `response.output_item.added` with `item.type: "reasoning"` — marks start of reasoning output
- `response.reasoning_summary_part.added` — marks start of a reasoning summary part
- `response.reasoning_summary_text.delta` — incremental reasoning text
- `response.reasoning_summary_text.done` — final reasoning text for a part
- `response.reasoning_summary_part.done` — end of a reasoning summary part
- `response.output_item.done` with `item.type: "reasoning"` — end of reasoning output

### Root Cause Analysis

Investigation revealed **two independent issues** contributing to reasoning summary loss:

**Issue A: `WithDefaults()` adds extraneous fields to stream events (PRIMARY)**

When raw passthrough is not enabled (the default), all stream events go through `WithDefaults()` which adds Bifrost-specific fields:
- `sequence_number: 0` on every event
- `extra_fields: {...}` with provider metadata
- Default `logprobs: []` arrays on text events
- Default `part` structures on content part events

OpenCode's `@ai-sdk/openai` SDK expects exact OpenAI wire format. These extra fields may cause the SDK parser to reject or misparse events, including reasoning summary events.

**Issue B: Missing accumulator handlers for reasoning events (SECONDARY)**

Bifrost's streaming accumulator handles `response.reasoning_summary_text.delta` but silently drops 5 other reasoning event types. This affects:
- Post-hooks that consume the accumulated response (logging, tracing, cost plugins)
- Non-OpenAI providers (Anthropic, Gemini) that don't have raw passthrough and rely on the accumulator → `WithDefaults()` path

### Key Finding: Raw Passthrough Exists But Is Not Enabled

Bifrost has a raw passthrough mechanism (`ExtraFields.RawResponse`) that preserves original OpenAI SSE events. However:
- It is **NOT** enabled by default for OpenAI Responses API requests
- It requires either `send_back_raw_response: true` in provider config or `x-bf-send-back-raw-response: true` HTTP header
- The Anthropic integration enables it automatically for Claude Code passthrough, but the OpenAI integration does not

### Key Finding: `encrypted_content` Location

`encrypted_content` is ONLY present in the `response.output_item.done` event's `item` field (inside `ResponsesReasoning.EncryptedContent`). It is NOT sent in `response.reasoning_summary_text.done` or any summary text event. Summary text events carry `signature` (not `encrypted_content`) at the event level.

## Goals / Non-Goals

**Goals:**
- Ensure OpenAI Responses API stream events reach clients in exact OpenAI wire format
- Preserve all reasoning summary events through Bifrost's Responses streaming path
- Ensure accumulated responses include complete reasoning data for post-hooks
- Maintain backward compatibility — no changes to existing working paths

**Non-Goals:**
- Fixing OpenCode's `openai-responses.ts` parser (separate issue, separate repo)
- Adding reasoning support to Chat Completions translation (already works for DeepSeek-style models)
- Changing the Responses-to-Chat fallback behavior (out of scope)

## Decisions

### D1: Enable raw passthrough for OpenAI Responses API by default

**Decision:** When the target provider is OpenAI and the request is Responses API, automatically enable raw response passthrough so that original OpenAI SSE events reach the client unchanged.

**Rationale:** The `WithDefaults()` method adds `sequence_number`, `extra_fields`, and default values that alter the wire format. OpenCode's `@ai-sdk/openai` SDK expects exact OpenAI format. Raw passthrough is the cleanest fix — it already exists in the codebase and is proven for Anthropic.

**Implementation:** In the OpenAI integration layer's `ResponsesStreamResponseConverter`, check for OpenAI provider and enable raw passthrough unconditionally (not just when `RawResponse != nil`). Alternatively, set `BifrostContextKeySendBackRawResponse` in the Responses API handler when provider is OpenAI.

**Alternatives considered:**
- *Strip extra fields from `WithDefaults()` output:* Rejected — fragile, would need to track every field OpenCode's SDK doesn't expect. Any new Bifrost metadata would risk breaking again.
- *Require users to set `send_back_raw_response: true` in config:* Rejected — poor UX, should work out of the box.

### D2: Add missing event handlers to the streaming accumulator

**Decision:** Add switch cases in `buildCompleteMessageFromResponsesStreamChunks` for all missing reasoning events.

**Rationale:** Even with raw passthrough for OpenAI, the accumulator is still needed for:
- Non-OpenAI providers (Anthropic, Gemini) that don't have raw passthrough
- Post-hooks (logging, tracing, cost) that consume the accumulated response
- The plugin pipeline which depends on complete accumulated data

**Alternatives considered:**
- *Skip accumulator, pass raw events through:* Rejected — the accumulator is needed for post-hook symmetry. Removing it would break the plugin pipeline.

### D3: Normalize reasoning accumulation to use ResponsesReasoning consistently

**Decision:** Route all reasoning summary deltas to `ResponsesReasoning.Summary` and `ResponsesReasoning.EncryptedContent`, regardless of `ContentIndex`. Content blocks with type `reasoning_text` should still be preserved as content blocks, but the primary reasoning text should also be in `ResponsesReasoning`.

**Rationale:** The `ResponsesReasoning` struct is the canonical location for reasoning data in the Responses schema. The `ContentIndex`-based routing creates ambiguity for downstream consumers.

## Risks / Trade-offs

- **Risk:** Enabling raw passthrough bypasses `WithDefaults()` which adds `extra_fields` metadata. Plugins that rely on `extra_fields` in stream events won't get them. → **Mitigation:** The accumulator still processes events internally; post-hooks still get complete data from the accumulated response. Only the client-facing stream is raw.
- **Risk:** Adding accumulator event handlers could affect performance for non-reasoning requests. → **Mitigation:** The switch statement is a simple type check; overhead is negligible (~nanoseconds).
- **Risk:** `encrypted_content` is only in `output_item.done`, not in summary text done events. If we try to extract it from summary done events, we'll find nothing. → **Mitigation:** Only extract `encrypted_content` from `output_item.done` events. Summary text events only carry `signature`.

## Migration Plan

No migration needed — this is a bug fix with no API changes. Deploy as a normal release.

## Open Questions

~~1. Does OpenAI's Responses API send `encrypted_content` at the stream event level?~~ → **RESOLVED:** No. `encrypted_content` is ONLY in `response.output_item.done`'s item. Summary text events carry `signature` only.

~~2. Should Bifrost pass through reasoning events as raw or accumulate only?~~ → **RESOLVED:** Both. Raw passthrough for client-facing stream (OpenAI provider), accumulation for post-hooks and non-OpenAI providers.
