package siliconflow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func newSFFileBatchVideoTestProvider(baseURL string) *SiliconFlowProvider {
	return &SiliconFlowProvider{
		client: &fasthttp.Client{
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		},
		networkConfig: schemas.NetworkConfig{BaseURL: baseURL},
	}
}

func sfFileBatchVideoTestContext() *schemas.BifrostContext {
	return schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
}

func sfFileBatchVideoTestKey() schemas.Key {
	return schemas.Key{Value: *schemas.NewSecretVar("test-key")}
}

func TestSiliconFlowFileStatusMappings(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   schemas.FileStatus
	}{
		{name: "processed", status: "processed", want: schemas.FileStatusProcessed},
		{name: "uploaded", status: "uploaded", want: schemas.FileStatusUploaded},
		{name: "error", status: "error", want: schemas.FileStatusError},
		{name: "unknown passes through", status: "processing", want: schemas.FileStatusProcessing},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, toBifrostFileStatus(tt.status))
		})
	}
}

func TestSiliconFlowFileObjectAcceptsTimestampVariants(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int64
	}{
		{name: "camel case upload timestamp", body: `{"id":"file-camel","createdAt":1710000001}`, want: 1710000001},
		{name: "snake case list timestamp", body: `{"id":"file-snake","created_at":1710000002}`, want: 1710000002},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var file SiliconFlowFileObject
			require.NoError(t, json.Unmarshal([]byte(tt.body), &file))
			assert.Equal(t, tt.want, file.createdAtUnix())
			assert.Equal(t, tt.want, file.ToFileObject().CreatedAt)
		})
	}
}

func TestSiliconFlowFileUploadMultipartAndResponseConversion(t *testing.T) {
	const fileContent = "{\"custom_id\":\"request-1\"}\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, fmt.Sprintf("unexpected method: %s", r.Method), http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/v1/files" {
			http.Error(w, fmt.Sprintf("unexpected path: %s", r.URL.Path), http.StatusNotFound)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			http.Error(w, fmt.Sprintf("unexpected authorization: %s", got), http.StatusUnauthorized)
			return
		}

		reader, err := r.MultipartReader()
		if err != nil {
			http.Error(w, fmt.Sprintf("read multipart body: %v", err), http.StatusBadRequest)
			return
		}

		parts := make(map[string]string)
		var filename string
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				http.Error(w, fmt.Sprintf("read multipart part: %v", err), http.StatusBadRequest)
				return
			}
			body, err := io.ReadAll(part)
			if err != nil {
				http.Error(w, fmt.Sprintf("read multipart content: %v", err), http.StatusBadRequest)
				return
			}
			parts[part.FormName()] = string(body)
			if part.FormName() == "file" {
				filename = part.FileName()
			}
		}

		if parts["purpose"] != "batch" {
			http.Error(w, fmt.Sprintf("unexpected purpose: %q", parts["purpose"]), http.StatusBadRequest)
			return
		}
		if filename != "requests.jsonl" || parts["file"] != fileContent {
			http.Error(w, fmt.Sprintf("unexpected file part: filename=%q body=%q", filename, parts["file"]), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"file-123","object":"file","bytes":30,"createdAt":1710000100,"filename":"requests.jsonl","purpose":"batch","status":"processed"}`))
	}))
	defer server.Close()

	provider := newSFFileBatchVideoTestProvider(server.URL)
	response, bifrostErr := provider.FileUpload(sfFileBatchVideoTestContext(), sfFileBatchVideoTestKey(), &schemas.BifrostFileUploadRequest{
		Provider: schemas.SiliconFlow,
		File:     []byte(fileContent),
		Filename: "requests.jsonl",
		Purpose:  schemas.FilePurposeBatch,
	})

	require.Nil(t, bifrostErr)
	require.NotNil(t, response)
	assert.Equal(t, "file-123", response.ID)
	assert.Equal(t, "file", response.Object)
	assert.Equal(t, int64(30), response.Bytes)
	assert.Equal(t, int64(1710000100), response.CreatedAt)
	assert.Equal(t, "requests.jsonl", response.Filename)
	assert.Equal(t, schemas.FilePurposeBatch, response.Purpose)
	assert.Equal(t, schemas.FileStatusProcessed, response.Status)
}

func TestSiliconFlowFileUploadRejectsNonBatchPurpose(t *testing.T) {
	provider := newSFFileBatchVideoTestProvider("")
	response, bifrostErr := provider.FileUpload(sfFileBatchVideoTestContext(), sfFileBatchVideoTestKey(), &schemas.BifrostFileUploadRequest{
		Provider: schemas.SiliconFlow,
		File:     []byte("content"),
		Filename: "data.jsonl",
		Purpose:  schemas.FilePurposeAssistants,
	})

	assert.Nil(t, response)
	require.NotNil(t, bifrostErr)
	require.NotNil(t, bifrostErr.Error)
	assert.Contains(t, bifrostErr.Error.Message, `only supports purpose "batch"`)
}
