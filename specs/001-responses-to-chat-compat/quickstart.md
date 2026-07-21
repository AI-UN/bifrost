# Quickstart: Responses To Chat Compatibility

## 1. Enable the feature in config

Set the new compatibility flag in `client_config.compat`:

```json
{
  "client_config": {
    "compat": {
      "convert_responses_to_chat": true
    }
  }
}
```

## 2. Save from the UI

Open `Workspace -> Config -> Compatibility`, enable `Convert Responses to Chat`, and save the configuration.

## 3. Validate non-streaming fallback

Send a Responses API request to a model that supports chat completions but not Responses. Confirm:

- the request succeeds
- the caller receives a Responses-format body
- response metadata indicates a converted request type

## 4. Validate streaming fallback

Send a streaming Responses API request to a chat-only model. Confirm:

- stream events arrive in Responses format
- the stream completes cleanly
- terminal metadata and usage are preserved when available

## 5. Validate per-request override

Disable the global setting, then send the same request with:

```text
x-bf-compat: ["convert_responses_to_chat"]
```

Confirm the fallback applies only to that request.
