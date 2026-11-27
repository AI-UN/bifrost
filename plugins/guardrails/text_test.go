package guardrails

import (
	"errors"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/require"
)

// collectFor collects one request's segments with a fresh context.
func collectFor(t *testing.T, req *schemas.BifrostRequest) *segmentSet {
	t.Helper()
	set, err := collectSegments(newTestContext(), req)
	require.NoError(t, err)
	return set
}

func TestCollectSegmentsChat(t *testing.T) {
	request := &schemas.BifrostRequest{
		RequestType: schemas.ChatCompletionRequest,
		ChatRequest: &schemas.BifrostChatRequest{
			Input: []schemas.ChatMessage{
				{
					Role:    schemas.ChatMessageRoleUser,
					Content: &schemas.ChatMessageContent{ContentStr: schemas.Ptr("user text")},
				},
				{
					Role: schemas.ChatMessageRoleUser,
					Content: &schemas.ChatMessageContent{ContentBlocks: []schemas.ChatContentBlock{
						{Type: schemas.ChatContentBlockTypeText, Text: schemas.Ptr("block text")},
						{Type: schemas.ChatContentBlockTypeImage, ImageURLStruct: &schemas.ChatInputImage{URL: "https://example.com/x.png"}},
					}},
				},
				{
					Role: schemas.ChatMessageRoleAssistant,
					ChatAssistantMessage: &schemas.ChatAssistantMessage{
						ToolCalls: []schemas.ChatAssistantMessageToolCall{
							{Function: schemas.ChatAssistantMessageToolCallFunction{Arguments: `{"query":"tool args"}`}},
							{Custom: &schemas.ChatAssistantMessageToolCallCustom{Input: "custom input"}},
						},
					},
				},
				{
					// A tool message's result is its content.
					Role:            schemas.ChatMessageRoleTool,
					Content:         &schemas.ChatMessageContent{ContentStr: schemas.Ptr("tool result")},
					ChatToolMessage: &schemas.ChatToolMessage{ToolCallID: schemas.Ptr("call_1")},
				},
			},
		},
	}

	require.Equal(t, []string{
		"user text",
		"block text",
		`{"query":"tool args"}`,
		"custom input",
		"tool result",
	}, collectFor(t, request).detectTexts())
}

func TestCollectSegmentsResponses(t *testing.T) {
	input := []schemas.ResponsesMessage{
		{Content: &schemas.ResponsesMessageContent{ContentStr: schemas.Ptr("message text")}},
		{Content: &schemas.ResponsesMessageContent{ContentBlocks: []schemas.ResponsesMessageContentBlock{
			{Type: schemas.ResponsesInputMessageContentBlockTypeText, Text: schemas.Ptr("block text")},
		}}},
		{ResponsesToolMessage: &schemas.ResponsesToolMessage{
			Arguments: schemas.Ptr(`{"q":"call args"}`),
			Output:    &schemas.ResponsesToolMessageOutputStruct{ResponsesToolCallOutputStr: schemas.Ptr("function output")},
		}},
		{ResponsesToolMessage: &schemas.ResponsesToolMessage{
			Output: &schemas.ResponsesToolMessageOutputStruct{ResponsesFunctionToolCallOutputBlocks: []schemas.ResponsesMessageContentBlock{
				{Type: schemas.ResponsesInputMessageContentBlockTypeText, Text: schemas.Ptr("output block")},
			}},
		}},
		// Reasoning the caller replays is forwarded to the provider, so it is scanned too.
		{ResponsesReasoning: &schemas.ResponsesReasoning{Summary: []schemas.ResponsesReasoningSummary{
			{Type: schemas.ResponsesReasoningContentBlockTypeSummaryText, Text: "replayed reasoning"},
		}}},
	}
	// Instructions are the caller's system prompt and lead the request.
	params := &schemas.ResponsesParameters{Instructions: schemas.Ptr("be terse")}
	expected := []string{
		"be terse", "message text", "block text", `{"q":"call args"}`, "function output", "output block", "replayed reasoning",
	}

	require.Equal(t, expected, collectFor(t, &schemas.BifrostRequest{
		RequestType:      schemas.ResponsesRequest,
		ResponsesRequest: &schemas.BifrostResponsesRequest{Input: input, Params: params},
	}).detectTexts())

	// CountTokens carries the same request shape and must be scanned the same way.
	require.Equal(t, expected, collectFor(t, &schemas.BifrostRequest{
		RequestType:        schemas.CountTokensRequest,
		CountTokensRequest: &schemas.BifrostResponsesRequest{Input: input, Params: params},
	}).detectTexts())
}

func TestCollectSegmentsChatSystemDeveloperAndReasoning(t *testing.T) {
	request := &schemas.BifrostRequest{
		RequestType: schemas.ChatCompletionRequest,
		ChatRequest: &schemas.BifrostChatRequest{Input: []schemas.ChatMessage{
			{Role: schemas.ChatMessageRoleSystem, Content: &schemas.ChatMessageContent{ContentStr: schemas.Ptr("system prompt")}},
			{Role: schemas.ChatMessageRoleDeveloper, Content: &schemas.ChatMessageContent{ContentStr: schemas.Ptr("developer prompt")}},
			{Role: schemas.ChatMessageRoleAssistant, ChatAssistantMessage: &schemas.ChatAssistantMessage{
				Reasoning: schemas.Ptr("replayed reasoning"),
				ReasoningDetails: []schemas.ChatReasoningDetails{
					{Type: schemas.BifrostReasoningDetailsTypeSummary, Summary: schemas.Ptr("reasoning summary")},
					{Type: schemas.BifrostReasoningDetailsTypeText, Text: schemas.Ptr("reasoning text")},
					{Type: schemas.BifrostReasoningDetailsTypeEncrypted, Data: schemas.Ptr("6f7061717565")},
				},
			}},
		}},
	}
	// Encrypted reasoning data is provider state, not readable text, and is left alone.
	require.Equal(t, []string{"system prompt", "developer prompt", "replayed reasoning", "reasoning summary", "reasoning text"},
		collectFor(t, request).detectTexts())
}

// A provider merges the caller's extra parameters into the JSON it sends after the typed request
// was inspected, so their string values must be inspected and rewritten too.
func TestCollectSegmentsExtraParams(t *testing.T) {
	nested := map[string]any{
		"prompt": "mail alex@example.com",
		"options": map[string]any{
			"tags": []any{"alex@example.com", 3.0},
		},
		"count": 2.0,
	}
	request := chatRequest("clean prompt")
	request.ChatRequest.Params = &schemas.ChatParameters{
		Metadata:    &map[string]any{"user": "alex@example.com"},
		ExtraParams: nested,
	}

	set := collectFor(t, request)
	require.Subset(t, set.detectTexts(), []string{"mail alex@example.com", "alex@example.com"})

	applied := set.rewrite(map[string]string{"alex@example.com": "[EMAIL_ADDRESS]"})
	require.Equal(t, map[string]string{"alex@example.com": "[EMAIL_ADDRESS]"}, applied)
	require.Equal(t, "mail [EMAIL_ADDRESS]", nested["prompt"])
	require.Equal(t, []any{"[EMAIL_ADDRESS]", 3.0}, nested["options"].(map[string]any)["tags"])
	require.Equal(t, "[EMAIL_ADDRESS]", (*request.ChatRequest.Params.Metadata)["user"])

	// Responses, text completion and compaction parameters are merged the same way.
	responses := &schemas.BifrostRequest{
		RequestType: schemas.ResponsesRequest,
		ResponsesRequest: &schemas.BifrostResponsesRequest{Params: &schemas.ResponsesParameters{
			ExtraParams: map[string]any{"instructions_suffix": "alex@example.com"},
			Metadata:    &map[string]any{"user_id": "alex@example.com"},
		}},
	}
	require.Subset(t, collectFor(t, responses).detectTexts(), []string{"alex@example.com"})
	collectFor(t, responses).rewrite(map[string]string{"alex@example.com": "[EMAIL_ADDRESS]"})
	require.Equal(t, "[EMAIL_ADDRESS]", responses.ResponsesRequest.Params.ExtraParams["instructions_suffix"])
	require.Equal(t, "[EMAIL_ADDRESS]", (*responses.ResponsesRequest.Params.Metadata)["user_id"])

	completion := &schemas.BifrostRequest{
		RequestType: schemas.TextCompletionRequest,
		TextCompletionRequest: &schemas.BifrostTextCompletionRequest{
			Input:  &schemas.TextCompletionInput{PromptStr: schemas.Ptr("prompt")},
			Params: &schemas.TextCompletionParameters{ExtraParams: map[string]any{"suffix": "alex@example.com"}},
		},
	}
	require.Contains(t, collectFor(t, completion).detectTexts(), "alex@example.com")

	compaction := &schemas.BifrostRequest{
		RequestType: schemas.CompactionRequest,
		CompactionRequest: &schemas.BifrostCompactionRequest{
			Instructions: schemas.Ptr("condense this"),
			ExtraParams:  map[string]any{"note": "alex@example.com"},
		},
	}
	require.Subset(t, collectFor(t, compaction).detectTexts(), []string{"condense this", "alex@example.com"})
}

// An inline batch carries each request's body, which is sent to the provider as it is.
func TestCollectSegmentsInlineBatch(t *testing.T) {
	request := &schemas.BifrostRequest{
		RequestType: schemas.BatchCreateRequest,
		BatchCreateRequest: &schemas.BifrostBatchCreateRequest{
			Requests: []schemas.BatchRequestItem{
				{Body: map[string]any{"messages": []any{map[string]any{"content": "mail alex@example.com"}}}},
				{CustomID: "two", Params: map[string]any{"prompt": "alex@example.com"}},
			},
			ExtraParams: map[string]any{"note": "alex@example.com"},
		},
	}

	set := collectFor(t, request)
	require.Equal(t, []string{"mail alex@example.com", "alex@example.com", "alex@example.com"}, set.detectTexts())

	applied := set.rewrite(map[string]string{"alex@example.com": "[EMAIL_ADDRESS]"})
	require.Equal(t, map[string]string{"alex@example.com": "[EMAIL_ADDRESS]"}, applied)
	body := request.BatchCreateRequest.Requests[0].Body["messages"].([]any)[0].(map[string]any)
	require.Equal(t, "mail [EMAIL_ADDRESS]", body["content"])
	require.Equal(t, "[EMAIL_ADDRESS]", request.BatchCreateRequest.Requests[1].Params["prompt"])
	require.Equal(t, "[EMAIL_ADDRESS]", request.BatchCreateRequest.ExtraParams["note"])
}

func TestCollectSegmentsRerank(t *testing.T) {
	request := &schemas.BifrostRequest{
		RequestType: schemas.RerankRequest,
		RerankRequest: &schemas.BifrostRerankRequest{
			Query: "who is alex@example.com",
			Documents: []schemas.RerankDocument{
				{Text: "alex@example.com lives here"},
				{Data: map[string]any{"email": "alex@example.com"}},
			},
			Params: &schemas.RerankParameters{ExtraParams: map[string]any{"note": "alex@example.com"}},
		},
	}

	set := collectFor(t, request)
	require.Equal(t, []string{
		"who is alex@example.com",
		"alex@example.com lives here",
		"alex@example.com",
		"alex@example.com",
	}, set.detectTexts())

	set.rewrite(map[string]string{"alex@example.com": "[EMAIL_ADDRESS]"})
	require.Equal(t, "who is [EMAIL_ADDRESS]", request.RerankRequest.Query)
	require.Equal(t, "[EMAIL_ADDRESS] lives here", request.RerankRequest.Documents[0].Text)
	require.Equal(t, "[EMAIL_ADDRESS]", request.RerankRequest.Documents[1].Data["email"])
	require.Equal(t, "[EMAIL_ADDRESS]", request.RerankRequest.Params.ExtraParams["note"])
}

func TestCollectSegmentsDecision(t *testing.T) {
	structured := map[string]any{"state": map[string]any{"owner": "alex@example.com"}}
	request := &schemas.BifrostRequest{
		RequestType: schemas.DecisionRequest,
		DecisionRequest: &schemas.BifrostDecisionRequest{
			Input: schemas.DecisionInput{
				Messages: []schemas.DecisionInputMessage{{
					Role:    "user",
					Content: schemas.DecisionInputContent{Parts: []schemas.DecisionInputPart{{Type: schemas.DecisionInputPartTypeText, Text: schemas.NewDecisionText("alex@example.com")}}},
					Input:   schemas.Ptr("tool alex@example.com"),
				}},
			},
			Questions: []schemas.DecisionQuestion{{
				Type:         schemas.DecisionTypeChoice,
				Instructions: schemas.NewDecisionText("decide for alex@example.com"),
				Criteria: &schemas.DecisionCriteria{
					True: &schemas.DecisionText{Structured: structured},
				},
				Choices: []schemas.DecisionChoice{{Description: schemas.NewDecisionText("alex@example.com choice")}},
				Levels:  []schemas.DecisionLevel{{Label: "high", Description: schemas.NewDecisionText("alex@example.com level")}},
			}},
			ExtraParams: map[string]any{"note": "alex@example.com"},
		},
	}

	set := collectFor(t, request)
	require.Equal(t, []string{
		"alex@example.com",
		"tool alex@example.com",
		"decide for alex@example.com",
		"alex@example.com choice",
		"alex@example.com level",
		"alex@example.com",
		"alex@example.com",
	}, set.detectTexts())

	set.rewrite(map[string]string{"alex@example.com": "[EMAIL_ADDRESS]"})
	message := request.DecisionRequest.Input.Messages[0]
	require.Equal(t, "[EMAIL_ADDRESS]", *message.Content.Parts[0].Text.Text)
	require.Equal(t, "tool [EMAIL_ADDRESS]", *message.Input)
	require.Equal(t, "decide for [EMAIL_ADDRESS]", *request.DecisionRequest.Questions[0].Instructions.Text)
	require.Equal(t, "[EMAIL_ADDRESS]",
		request.DecisionRequest.Questions[0].Criteria.True.Structured.(map[string]any)["state"].(map[string]any)["owner"])
	require.Equal(t, "[EMAIL_ADDRESS] choice", *request.DecisionRequest.Questions[0].Choices[0].Description.Text)
	require.Equal(t, "[EMAIL_ADDRESS] level", *request.DecisionRequest.Questions[0].Levels[0].Description.Text)
	require.Equal(t, "[EMAIL_ADDRESS]", request.DecisionRequest.ExtraParams["note"])
}

func TestCollectSegmentsCachedContent(t *testing.T) {
	request := &schemas.BifrostRequest{
		RequestType: schemas.CachedContentCreateRequest,
		CachedContentCreateRequest: &schemas.BifrostCachedContentCreateRequest{
			SystemInstruction: map[string]any{"parts": []any{map[string]any{"text": "be terse for alex@example.com"}}},
			Contents:          []any{map[string]any{"parts": []any{map[string]any{"text": "alex@example.com wrote this"}}}},
			ToolConfig:        "alex@example.com",
			ExtraParams:       map[string]any{"note": "alex@example.com"},
		},
	}

	set := collectFor(t, request)
	require.Equal(t, []string{
		"alex@example.com wrote this",
		"alex@example.com",
		"be terse for alex@example.com",
		"alex@example.com",
	}, set.detectTexts())

	set.rewrite(map[string]string{"alex@example.com": "[EMAIL_ADDRESS]"})
	require.Equal(t, "be terse for [EMAIL_ADDRESS]",
		request.CachedContentCreateRequest.SystemInstruction.(map[string]any)["parts"].([]any)[0].(map[string]any)["text"])
	require.Equal(t, "[EMAIL_ADDRESS] wrote this",
		request.CachedContentCreateRequest.Contents[0].(map[string]any)["parts"].([]any)[0].(map[string]any)["text"],
		"a cached content slice element is rewritten in place")
	// A bare string in an any-typed field is assigned back through the field.
	require.Equal(t, "[EMAIL_ADDRESS]", request.CachedContentCreateRequest.ToolConfig)
	require.Equal(t, "[EMAIL_ADDRESS]", request.CachedContentCreateRequest.ExtraParams["note"])
}

func TestCollectSegmentsCompactionRequest(t *testing.T) {
	request := &schemas.BifrostRequest{
		RequestType: schemas.CompactionRequest,
		CompactionRequest: &schemas.BifrostCompactionRequest{
			Instructions:       schemas.Ptr("condense this"),
			Input:              []schemas.ResponsesMessage{{Content: &schemas.ResponsesMessageContent{ContentStr: schemas.Ptr("mail alex@example.com")}}},
			PreviousResponseID: schemas.Ptr("resp_1"),
		},
	}
	require.Equal(t, []string{"mail alex@example.com", "condense this"}, collectFor(t, request).detectTexts())
}

// A passthrough body is inspected as the JSON the provider reads, not as the caller's bytes: an
// escape must not hide a value.
func TestCollectSegmentsPassthroughJSON(t *testing.T) {
	body := []byte(`{"input":"mail alex\u0040example.com","n":1}`)
	request := &schemas.BifrostRequest{
		RequestType:        schemas.PassthroughRequest,
		PassthroughRequest: &schemas.BifrostPassthroughRequest{Body: body},
	}

	set := collectFor(t, request)
	require.Equal(t, []string{"mail alex@example.com"}, set.detectTexts())

	// The bytes belong to the integration's rewriter, so rewrite reports the applied literal and
	// leaves the body alone.
	applied := set.rewrite(map[string]string{"alex@example.com": "[EMAIL_ADDRESS]"})
	require.Equal(t, map[string]string{"alex@example.com": "[EMAIL_ADDRESS]"}, applied)
	require.Equal(t, body, request.PassthroughRequest.Body)
}

// A multipart passthrough supplies its text as form fields. File parts are media and stay out.
func TestCollectSegmentsPassthroughMultipart(t *testing.T) {
	body := strings.Join([]string{
		"--b\r\nContent-Disposition: form-data; name=\"prompt\"\r\n\r\nmail alex@example.com\r\n",
		"--b\r\nContent-Disposition: form-data; name=\"n\"\r\n\r\n1\r\n",
		"--b\r\nContent-Disposition: form-data; name=\"file\"; filename=\"a.txt\"\r\nContent-Type: text/plain\r\n\r\nalex@example.com in a file\r\n",
		"--b--\r\n",
	}, "")
	form, err := multipart.NewReader(strings.NewReader(body), "b").ReadForm(1 << 20)
	require.NoError(t, err)
	t.Cleanup(func() { _ = form.RemoveAll() })

	request := &schemas.BifrostRequest{
		RequestType: schemas.PassthroughRequest,
		PassthroughRequest: &schemas.BifrostPassthroughRequest{
			Form:        form,
			BodySource:  schemas.NewBytesBodySource([]byte("stale")),
			SafeHeaders: map[string]string{"content-type": "multipart/form-data; boundary=b"},
		},
	}

	set := collectFor(t, request)
	require.Equal(t, []string{"1", "mail alex@example.com"}, set.detectTexts(),
		"file contents are a media exception")

	set.rewrite(map[string]string{"alex@example.com": "[EMAIL_ADDRESS]"})
	require.Equal(t, "mail [EMAIL_ADDRESS]", form.Value["prompt"][0])
}

func TestCollectSegmentsUnsupportedShapes(t *testing.T) {
	t.Run("opaque stream", func(t *testing.T) {
		set, err := collectSegments(newTestContext(), &schemas.BifrostRequest{
			RequestType:        schemas.PassthroughRequest,
			PassthroughRequest: &schemas.BifrostPassthroughRequest{BodySource: schemas.NewBytesBodySource([]byte("x"))},
		})
		require.Nil(t, set)
		require.True(t, errors.Is(err, errUnsupportedShape))
		require.Contains(t, err.Error(), string(schemas.PassthroughRequest), "the error names the request type")
	})

	t.Run("body that is not JSON", func(t *testing.T) {
		set, err := collectSegments(newTestContext(), &schemas.BifrostRequest{
			RequestType:        schemas.PassthroughRequest,
			PassthroughRequest: &schemas.BifrostPassthroughRequest{Body: []byte("input=alex@example.com")},
		})
		require.Nil(t, set)
		require.True(t, errors.Is(err, errUnsupportedShape))
	})

	t.Run("raw body that is not JSON", func(t *testing.T) {
		ctx := newTestContext()
		ctx.SetValue(schemas.BifrostContextKeyUseRawRequestBody, true)
		request := chatRequest("hi")
		request.ChatRequest.RawRequestBody = []byte("not json")
		_, err := collectSegments(ctx, request)
		require.True(t, errors.Is(err, errUnsupportedShape))
	})
}

// A responses item Bifrost re-emits from its preserved bytes must be rewritten there too, or the
// original value rides along in the bytes the provider receives.
func TestCollectSegmentsPreservedResponsesItem(t *testing.T) {
	var message schemas.ResponsesMessage
	require.NoError(t, schemas.Unmarshal([]byte(
		`{"type":"tool_search_call","id":"ts_1","status":"completed","call_id":"c1","execution":"client","arguments":{"query":"mail alex@example.com"}}`,
	), &message))
	require.True(t, message.IsVerbatimItem())

	request := &schemas.BifrostRequest{
		RequestType:      schemas.ResponsesRequest,
		ResponsesRequest: &schemas.BifrostResponsesRequest{Input: []schemas.ResponsesMessage{message}},
	}
	set := collectFor(t, request)
	require.Contains(t, set.detectTexts(), `{"query":"mail alex@example.com"}`)

	applied := set.rewrite(map[string]string{"alex@example.com": "[EMAIL_ADDRESS]"})
	require.Equal(t, map[string]string{"alex@example.com": "[EMAIL_ADDRESS]"}, applied)

	raw, err := schemas.Marshal(request.ResponsesRequest.Input[0])
	require.NoError(t, err)
	require.Contains(t, string(raw), "[EMAIL_ADDRESS]")
	require.NotContains(t, string(raw), "alex@example.com")
	require.NotContains(t, string(raw), `\u0040`)
}

// A custom tool call's input and a compaction summary are replayed text, so both are inspected.
func TestCollectSegmentsResponsesCustomToolAndCompaction(t *testing.T) {
	request := &schemas.BifrostRequest{
		RequestType: schemas.ResponsesRequest,
		ResponsesRequest: &schemas.BifrostResponsesRequest{Input: []schemas.ResponsesMessage{
			{ResponsesToolMessage: &schemas.ResponsesToolMessage{
				ResponsesCustomToolCall: &schemas.ResponsesCustomToolCall{Input: "mail alex@example.com"},
			}},
			{Content: &schemas.ResponsesMessageContent{ContentBlocks: []schemas.ResponsesMessageContentBlock{{
				Type: schemas.ResponsesOutputMessageContentTypeCompaction,
				ResponsesOutputMessageContentCompaction: &schemas.ResponsesOutputMessageContentCompaction{
					Summary: "alex@example.com asked about billing",
				},
			}}}},
		}},
	}

	set := collectFor(t, request)
	require.Equal(t, []string{"mail alex@example.com", "alex@example.com asked about billing"}, set.detectTexts())

	set.rewrite(map[string]string{"alex@example.com": "[EMAIL_ADDRESS]"})
	require.Equal(t, "mail [EMAIL_ADDRESS]", request.ResponsesRequest.Input[0].ResponsesToolMessage.ResponsesCustomToolCall.Input)
	require.Equal(t, "[EMAIL_ADDRESS] asked about billing",
		request.ResponsesRequest.Input[1].Content.ContentBlocks[0].Summary)
}

// When the raw body is forwarded, the fields only it carries (such as an Anthropic
// metadata.user_id) are inspected as well.
func TestCollectSegmentsRawBodyWhenForwarded(t *testing.T) {
	request := chatRequest("clean prompt")
	request.ChatRequest.RawRequestBody = []byte(`{"messages":[{"content":"clean"}],"metadata":{"user_id":"alex@example.com"}}`)

	off := newTestContext()
	require.NotContains(t, mustCollectWith(t, off, request).detectTexts(), "alex@example.com",
		"without raw forwarding the native-only field is never sent")

	on := newTestContext()
	on.SetValue(schemas.BifrostContextKeyUseRawRequestBody, true)
	set, err := collectSegments(on, request)
	require.NoError(t, err)
	require.Contains(t, set.detectTexts(), "alex@example.com")
	require.Contains(t, set.detectTexts(), "clean")
}

func mustCollectWith(t *testing.T, ctx *schemas.BifrostContext, req *schemas.BifrostRequest) *segmentSet {
	t.Helper()
	set, err := collectSegments(ctx, req)
	require.NoError(t, err)
	return set
}

func TestCollectSegmentsOtherRequestTypes(t *testing.T) {
	t.Run("text completion", func(t *testing.T) {
		request := &schemas.BifrostRequest{
			RequestType: schemas.TextCompletionRequest,
			TextCompletionRequest: &schemas.BifrostTextCompletionRequest{
				Input: &schemas.TextCompletionInput{PromptStr: schemas.Ptr("prompt")},
			},
		}
		require.Equal(t, []string{"prompt"}, collectFor(t, request).detectTexts())

		request.TextCompletionRequest.Input = &schemas.TextCompletionInput{PromptArray: []string{"first", "second"}}
		require.Equal(t, []string{"first", "second"}, collectFor(t, request).detectTexts())
	})

	t.Run("embedding", func(t *testing.T) {
		request := &schemas.BifrostRequest{
			RequestType: schemas.EmbeddingRequest,
			EmbeddingRequest: &schemas.BifrostEmbeddingRequest{Input: []schemas.EmbeddingInputItem{
				{Content: schemas.EmbeddingContent{
					{Type: schemas.EmbeddingContentPartTypeText, Text: schemas.Ptr("embedded text")},
					{Type: schemas.EmbeddingContentPartTypeImage, Image: &schemas.EmbeddingMediaPart{URL: schemas.Ptr("https://example.com/x.png")}},
				}},
			}},
		}
		require.Equal(t, []string{"embedded text"}, collectFor(t, request).detectTexts())
	})

	t.Run("image generation", func(t *testing.T) {
		request := &schemas.BifrostRequest{
			RequestType:            schemas.ImageGenerationRequest,
			ImageGenerationRequest: &schemas.BifrostImageGenerationRequest{Input: &schemas.ImageGenerationInput{Prompt: "draw a cat"}},
		}
		require.Equal(t, []string{"draw a cat"}, collectFor(t, request).detectTexts())
	})

	t.Run("image edit", func(t *testing.T) {
		request := &schemas.BifrostRequest{
			RequestType:      schemas.ImageEditRequest,
			ImageEditRequest: &schemas.BifrostImageEditRequest{Input: &schemas.ImageEditInput{Prompt: "edit this"}},
		}
		require.Equal(t, []string{"edit this"}, collectFor(t, request).detectTexts())
	})

	t.Run("speech", func(t *testing.T) {
		request := &schemas.BifrostRequest{
			RequestType:   schemas.SpeechRequest,
			SpeechRequest: &schemas.BifrostSpeechRequest{Input: &schemas.SpeechInput{Input: "say this"}},
		}
		require.Equal(t, []string{"say this"}, collectFor(t, request).detectTexts())
	})

	t.Run("transcription prompt", func(t *testing.T) {
		request := &schemas.BifrostRequest{
			RequestType: schemas.TranscriptionRequest,
			TranscriptionRequest: &schemas.BifrostTranscriptionRequest{
				Params: &schemas.TranscriptionParameters{Prompt: schemas.Ptr("a prompt")},
			},
		}
		require.Equal(t, []string{"a prompt"}, collectFor(t, request).detectTexts())
	})

	t.Run("OCR annotation prompt", func(t *testing.T) {
		request := &schemas.BifrostRequest{
			RequestType: schemas.OCRRequest,
			OCRRequest: &schemas.BifrostOCRRequest{
				Params: &schemas.OCRParameters{DocumentAnnotationPrompt: schemas.Ptr("describe the table")},
			},
		}
		require.Equal(t, []string{"describe the table"}, collectFor(t, request).detectTexts())
	})

	t.Run("video prompt", func(t *testing.T) {
		request := &schemas.BifrostRequest{
			RequestType: schemas.VideoGenerationRequest,
			VideoGenerationRequest: &schemas.BifrostVideoGenerationRequest{
				Input: &schemas.VideoGenerationInput{Prompt: "a clip"},
			},
		}
		require.Equal(t, []string{"a clip"}, collectFor(t, request).detectTexts())
	})
}

func TestCollectSegmentsSkipsEmptyText(t *testing.T) {
	request := chatRequest("")
	request.ChatRequest.Input[0].Content.ContentBlocks = []schemas.ChatContentBlock{{Type: schemas.ChatContentBlockTypeImage}}
	require.Empty(t, collectFor(t, request).detectTexts())
}

// A request type with no user text carries none to inspect, and neither does a nil request.
func TestCollectSegmentsTextlessShapes(t *testing.T) {
	set, err := collectSegments(newTestContext(), nil)
	require.NoError(t, err)
	require.Nil(t, set)
	require.Nil(t, set.detectTexts())
	count, bytes, sum := set.fingerprint()
	require.Zero(t, count)
	require.Zero(t, bytes)
	require.Equal(t, uint64(fnvOffset64), sum)

	set, err = collectSegments(newTestContext(), &schemas.BifrostRequest{
		RequestType:       schemas.ListModelsRequest,
		ListModelsRequest: &schemas.BifrostListModelsRequest{Provider: schemas.OpenAI},
	})
	require.NoError(t, err)
	require.Empty(t, set.detectTexts())
	require.Empty(t, set.rewrite(map[string]string{"a": "b"}))
}

// Overlapping matches are replaced as one span: replacing them one at a time can leave the part of
// a value that no single literal covered.
func TestRewriteMergesOverlappingSpans(t *testing.T) {
	request := chatRequest("John Smithers Street")
	replacements := map[string]string{
		"John Smithers":   "[PERSON]",
		"Smithers Street": "[LOCATION]",
	}

	applied := collectFor(t, request).rewrite(replacements)
	require.Equal(t, "[PERSON]", *request.ChatRequest.Input[0].Content.ContentStr,
		"a merged span keeps the placeholder of the earliest-starting match")
	require.Equal(t, map[string]string{"John Smithers Street": "[PERSON]"}, applied,
		"the merged span is published too, so logging redacts it")

	// A value crossing a shorter one is still replaced whole, whichever order they arrive in.
	crossing := chatRequest("name: alex@example.com and street")
	collectFor(t, crossing).rewrite(map[string]string{
		"alex@example.com and": "[MATCH_A]",
		"com and street":       "[MATCH_B]",
	})
	require.Equal(t, "name: [MATCH_A]", *crossing.ChatRequest.Input[0].Content.ContentStr)
}

func TestRewriteReturnsEmptyMapForNoReplacements(t *testing.T) {
	request := chatRequest("untouched")
	set := collectFor(t, request)
	applied := set.rewrite(map[string]string{})
	require.NotNil(t, applied)
	require.Empty(t, applied)
	require.Equal(t, "untouched", *request.ChatRequest.Input[0].Content.ContentStr)
}

func TestFingerprintTracksInspectedText(t *testing.T) {
	request := chatRequest("hello")
	count, bytes, sum := collectFor(t, request).fingerprint()
	require.Equal(t, 1, count)
	require.Equal(t, len("hello"), bytes)

	request.ChatRequest.Input = append(request.ChatRequest.Input, schemas.ChatMessage{
		Content: &schemas.ChatMessageContent{ContentStr: schemas.Ptr("more")},
	})
	longer, moreBytes, moreSum := collectFor(t, request).fingerprint()
	require.Equal(t, 2, longer)
	require.Equal(t, bytes+len("more"), moreBytes)
	require.NotEqual(t, sum, moreSum)

	// A swap the count and byte total cannot see still changes the hash.
	swapped := chatRequest("olleh")
	_, sameBytes, swappedSum := collectFor(t, swapped).fingerprint()
	require.Equal(t, bytes, sameBytes)
	require.NotEqual(t, sum, swappedSum)

	// Segments are separated, so text split differently does not hash the same.
	split := chatRequest("hel")
	split.ChatRequest.Input = append(split.ChatRequest.Input, schemas.ChatMessage{
		Content: &schemas.ChatMessageContent{ContentStr: schemas.Ptr("lo")},
	})
	_, splitBytes, splitSum := collectFor(t, split).fingerprint()
	require.Equal(t, bytes, splitBytes)
	require.NotEqual(t, sum, splitSum)

	// The same text inspected twice gives the same identity.
	_, _, again := collectFor(t, chatRequest("hello")).fingerprint()
	require.Equal(t, sum, again)

	// The fingerprint follows the raw body only when it is forwarded.
	raw := chatRequest("hello")
	raw.ChatRequest.RawRequestBody = []byte(`{"messages":[{"content":"hello"}]}`)
	rawOff := newTestContext()
	_, _, offSum := mustCollectWith(t, rawOff, raw).fingerprint()
	require.Equal(t, sum, offSum, "the raw body is ignored unless it is forwarded")
	rawOn := newTestContext()
	rawOn.SetValue(schemas.BifrostContextKeyUseRawRequestBody, true)
	_, _, withRaw := mustCollectWith(t, rawOn, raw).fingerprint()
	require.NotEqual(t, sum, withRaw)
	_, _, withRawAgain := mustCollectWith(t, rawOn, raw).fingerprint()
	require.Equal(t, withRaw, withRawAgain)
}
