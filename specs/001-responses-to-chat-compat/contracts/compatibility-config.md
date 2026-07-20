# Contract: Responses To Chat Compatibility

## Config Contract

The client configuration payload gains a new boolean field under `client_config.compat`:

```json
{
  "client_config": {
    "compat": {
      "convert_responses_to_chat": false
    }
  }
}
```

### Rules

- Default value is `false`.
- The field must be accepted by config schema validation.
- The field must be returned by config read endpoints.
- Updating the field must participate in the same compat plugin reload flow as the existing compatibility flags.

## Header Override Contract

The `x-bf-compat` request header accepts the new feature name:

```text
x-bf-compat: ["convert_responses_to_chat"]
```

### Rules

- `x-bf-compat: true` and `x-bf-compat: ["*"]` must also enable the new feature.
- The override only affects the current request.
- The override does not force conversion unless the model-catalog capability check says chat is supported and Responses is not.

## Behavioral Contract

When the feature is active for a request:

- A non-streaming Responses request may be sent upstream as a chat-completions request.
- A streaming Responses request may be sent upstream as a chat-completions streaming request.
- The caller must still receive Responses-format output.
- Response metadata must continue to indicate the original caller-facing request type and the converted upstream type.
