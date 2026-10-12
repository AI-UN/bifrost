package siliconflow

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSiliconFlowBatchStatusInQueueMapsToValidating(t *testing.T) {
	assert.Equal(t, schemas.BatchStatusValidating, toBifrostBatchStatus("in_queue"))
}

func TestSiliconFlowBatchErrorConversion(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []schemas.BatchError
	}{
		{name: "nil", in: nil, want: nil},
		{name: "empty", in: []string{}, want: nil},
		{name: "only empty messages", in: []string{"", ""}, want: nil},
		{name: "nonempty messages", in: []string{"first", "", "second"}, want: []schemas.BatchError{{Message: "first"}, {Message: "second"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toBifrostBatchErrors(tt.in)
			if tt.want == nil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, tt.want, got.Data)
		})
	}
}

func TestSiliconFlowBatchResponseErrorsJSON(t *testing.T) {
	tests := []struct {
		name string
		body string
		want *schemas.BatchErrors
	}{
		{name: "null", body: `{"id":"batch-null","status":"failed","errors":null}`, want: nil},
		{name: "string array", body: `{"id":"batch-errors","status":"failed","errors":["a"]}`, want: &schemas.BatchErrors{Data: []schemas.BatchError{{Message: "a"}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var response SiliconFlowBatchResponse
			require.NoError(t, json.Unmarshal([]byte(tt.body), &response))
			converted := response.ToBifrostBatchRetrieveResponse(0, false, false, nil, nil)
			assert.Equal(t, tt.want, converted.Errors)
		})
	}
}

func TestSiliconFlowBatchCreateUsesChatEndpointModelReplacementAndDefaultWindow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, fmt.Sprintf("unexpected method: %s", r.Method), http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/v1/batches" {
			http.Error(w, fmt.Sprintf("unexpected path: %s", r.URL.Path), http.StatusNotFound)
			return
		}

		var body SiliconFlowBatchRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, fmt.Sprintf("decode body: %v", err), http.StatusBadRequest)
			return
		}
		if body.InputFileID != "file-123" {
			http.Error(w, fmt.Sprintf("unexpected input_file_id: %q", body.InputFileID), http.StatusBadRequest)
			return
		}
		if body.Endpoint != string(schemas.BatchEndpointChatCompletions) {
			http.Error(w, fmt.Sprintf("unexpected endpoint: %q", body.Endpoint), http.StatusBadRequest)
			return
		}
		if body.CompletionWindow != "24h" {
			http.Error(w, fmt.Sprintf("unexpected completion window: %q", body.CompletionWindow), http.StatusBadRequest)
			return
		}
		if body.Replace == nil || body.Replace.Model != "Qwen/Qwen3-8B" {
			http.Error(w, fmt.Sprintf("unexpected replace: %#v", body.Replace), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"batch-123","object":"batch","endpoint":"/v1/chat/completions","input_file_id":"file-123","completion_window":"24h","status":"in_queue","created_at":1710000200,"request_counts":{"total":3,"completed":0,"failed":0}}`))
	}))
	defer server.Close()

	model := "Qwen/Qwen3-8B"
	provider := newSFFileBatchVideoTestProvider(server.URL)
	response, bifrostErr := provider.BatchCreate(sfFileBatchVideoTestContext(), sfFileBatchVideoTestKey(), &schemas.BifrostBatchCreateRequest{
		Provider:    schemas.SiliconFlow,
		Model:       &model,
		InputFileID: "file-123",
	})

	require.Nil(t, bifrostErr)
	require.NotNil(t, response)
	assert.Equal(t, "batch-123", response.ID)
	assert.Equal(t, "batch", response.Object)
	assert.Equal(t, string(schemas.BatchEndpointChatCompletions), response.Endpoint)
	assert.Equal(t, "file-123", response.InputFileID)
	assert.Equal(t, "24h", response.CompletionWindow)
	assert.Equal(t, schemas.BatchStatusValidating, response.Status)
	assert.Equal(t, schemas.BatchRequestCounts{Total: 3}, response.RequestCounts)
	assert.Equal(t, int64(1710000200), response.CreatedAt)
}

func TestSiliconFlowBatchCreateRejectsConflictingAndEmptyInputs(t *testing.T) {
	provider := newSFFileBatchVideoTestProvider("")

	t.Run("conflicting input file and requests", func(t *testing.T) {
		response, bifrostErr := provider.BatchCreate(sfFileBatchVideoTestContext(), sfFileBatchVideoTestKey(), &schemas.BifrostBatchCreateRequest{
			Provider:    schemas.SiliconFlow,
			InputFileID: "file-123",
			Requests: []schemas.BatchRequestItem{
				{CustomID: "request-1", Method: http.MethodPost, URL: string(schemas.BatchEndpointChatCompletions)},
			},
		})

		assert.Nil(t, response)
		require.NotNil(t, bifrostErr)
		require.NotNil(t, bifrostErr.Error)
		assert.Contains(t, bifrostErr.Error.Message, "either input_file_id or requests, not both")
	})

	t.Run("all inputs empty", func(t *testing.T) {
		response, bifrostErr := provider.BatchCreate(sfFileBatchVideoTestContext(), sfFileBatchVideoTestKey(), &schemas.BifrostBatchCreateRequest{
			Provider: schemas.SiliconFlow,
		})

		assert.Nil(t, response)
		require.NotNil(t, bifrostErr)
		require.NotNil(t, bifrostErr.Error)
		assert.Contains(t, bifrostErr.Error.Message, "either input_file_id or requests array is required")
	})
}
