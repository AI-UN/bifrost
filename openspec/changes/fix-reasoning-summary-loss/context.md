# Bifrost Reasoning Summary Handoff

## Context

We are investigating a reasoning-summary display failure across multiple OpenCode clients when they route through Bifrost.

## Symptom

- OpenCode over Bifrost does not show reasoning summaries for OpenAI/Claude/Codex models.
- Claude Code over Bifrost also shows the same issue for Claude models.
- Codex over Bifrost also shows the same issue for GPT models.
- Models using Chat Completions directly (for example DeepSeek) still show reasoning summaries normally.
- Direct CPA/CLIProxyAPI without Bifrost shows reasoning summaries normally.

## Current Conclusion

This looks much more like a Bifrost-side compatibility/translation issue than an OpenCode-only problem.

The strongest evidence is:

1. The same client behavior works when Bifrost is removed.
2. The same client behavior fails across multiple clients when Bifrost is inserted.
3. Chat Completions-based models still show summaries normally.

## Verified Config State

Current OpenCode config (`/home/evans/.config/opencode/opencode.jsonc`):

```jsonc
"openai": {
  "npm": "@ai-sdk/openai",
  "name": "AI Gateway",
  "options": {
    "baseURL": "https://ai-gateway.mkvs.eu.org/openai",
    "setCacheKey": true,
    "reasoningEffort": "medium",
    "reasoningSummary": "auto",
    "textVerbosity": "medium",
    "include": ["reasoning.encrypted_content"],
    "store": false
  }
}
```

Direct CPA/CLIProxyAPI config in the same workspace also uses `@ai-sdk/openai` and similar reasoning options, but it works when Bifrost is not in the path.

Relevant direct CPA provider entry from the backup config (`/home/evans/.config/opencode/opencode.bak.jsonc`):

```jsonc
"cpa-codex": {
  "npm": "@ai-sdk/openai",
  "name": "CLI Proxy API (Codex)",
  "options": {
    "baseURL": "https://cli-proxy.mkvs.eu.org/v1",
    "setCacheKey": true,
    "reasoningEffort": "medium",
    "reasoningSummary": "auto",
    "textVerbosity": "medium",
    "include": ["reasoning.encrypted_content"],
    "store": false
  }
}
```

## Important Observation

When I tried switching OpenCode to `@ai-sdk/openai-compatible`, I hit:

`Z.responses is not a function. (In 'Z.responses(Q)', 'Z.responses' is undefined)`

That seems to be a separate compatibility problem and should not be confused with the main reasoning-summary issue.

## Relevant Repos

- OpenCode clone: `/home/evans/Projects/Personal/opencode` (or current workspace refs in prior inspection)
- CLIProxyAPI clone: local workspace clone exists
- Bifrost fork: local clone exists and is the next investigation target

## OpenCode Evidence

OpenCode has a known gap in `packages/llm/src/protocols/openai-responses.ts`:

- It handles `response.output_text.delta`, tool calls, completion, etc.
- It also supports reasoning-related request knobs.
- But the stream parser does not fully map reasoning summary events into the UI path.

OpenCode also has a separate working path for OpenAI-compatible chat models in `packages/llm/src/protocols/openai-chat.ts`, which explains why Chat Completions-style models can still display reasoning better.

## Bifrost Evidence / Code Paths To Inspect

Please inspect these files first in the Bifrost fork:

- `core/providers/openai/responses.go`
- `framework/streaming/responses.go`
- `core/schemas/responses.go`
- `core/providers/openai/chat.go`
- `docs/providers/reasoning.mdx`
- `docs/cli-agents/opencode.mdx`

Likely question to answer:

- Does Bifrost preserve and emit reasoning summary blocks consistently when routing OpenAI Responses through the gateway?
- Does Bifrost transform some upstream reasoning shape into a form that OpenCode does not consume?
- Are `reasoning.summary`, `reasoning.encrypted_content`, or stream events like `response.reasoning_summary_text.delta` being lost, normalized, or mis-tagged?

## Related Bifrost Issues / PRs

Most relevant existing references:

- `#3378` - Add bidirectional conversion between Responses, Messages, and Chat Completions APIs
- `#2599` - add responses to chat fallback for custom openai providers
- `#1977` - Chat-to-Responses mux does not create reasoning output items from assistant reasoning content
- `#1149` - Claude Extended Thinking signature Field Not Preserved in OpenAI Format Translation
- `#3138` - Bifrost injects reasoning into chat completion responses for models without reasoning support
- `#567` - Handling reasoning content

## Reproduction Matrix

Works:

- Direct CLIProxyAPI / CPA provider, no Bifrost
- Chat Completions models (for example DeepSeek)

Fails:

- OpenCode -> Bifrost -> OpenAI / Claude / Codex models
- Claude Code -> Bifrost -> Claude models
- Codex -> Bifrost -> GPT models

## Suggested Next Steps In Bifrost Fork

1. Capture raw request and response payloads for one failing model through Bifrost and one working direct-CPA path.
2. Compare event shapes for reasoning across the two paths.
3. Check whether Bifrost is sending/returning:
   - `reasoning.summary`
   - `reasoning.encrypted_content`
   - `reasoning_details`
   - `response.reasoning_summary_part.added`
   - `response.reasoning_summary_text.delta`
4. Confirm whether Bifrost is converting Responses -> Chat or Responses -> Responses in the failing path.
5. If the payload is correct, the next target is OpenCode’s stream parser. If the payload is missing/mutated, fix Bifrost first.

## Short Hypothesis

The failure is most likely in Bifrost’s OpenAI integration layer or its Responses/Chat translation path, not in the default OpenAI provider itself.
