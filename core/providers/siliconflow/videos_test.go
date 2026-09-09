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

func TestSiliconFlowVideoStatusMappings(t *testing.T) {
	tests := []struct {
		name    string
		status  string
		want    schemas.VideoStatus
		wantErr string
	}{
		{name: "in queue", status: "InQueue", want: schemas.VideoStatusQueued},
		{name: "in progress", status: "InProgress", want: schemas.VideoStatusInProgress},
		{name: "succeed", status: "Succeed", want: schemas.VideoStatusCompleted},
		{name: "failed", status: "Failed", want: schemas.VideoStatusFailed},
		{name: "unknown", status: "Paused", wantErr: `unknown SiliconFlow video status "Paused"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toBifrostVideoStatus(tt.status)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSiliconFlowVideoSubmitRequiresPrompt(t *testing.T) {
	tests := []struct {
		name    string
		request *schemas.BifrostVideoGenerationRequest
	}{
		{name: "nil request", request: nil},
		{name: "nil input", request: &schemas.BifrostVideoGenerationRequest{Model: "Wan-AI/Wan2.2-T2V-A14B"}},
		{name: "blank prompt", request: &schemas.BifrostVideoGenerationRequest{Model: "Wan-AI/Wan2.2-T2V-A14B", Input: &schemas.VideoGenerationInput{Prompt: "  "}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converted, err := ToSiliconFlowVideoSubmitRequest(tt.request)
			assert.Nil(t, converted)
			require.EqualError(t, err, "video generation prompt is required")
		})
	}
}

func TestSiliconFlowVideoSubmitMapsInputReferenceToImage(t *testing.T) {
	inputReference := "data:image/png;base64,aW1hZ2U="
	converted, err := ToSiliconFlowVideoSubmitRequest(&schemas.BifrostVideoGenerationRequest{
		Provider: schemas.SiliconFlow,
		Model:    "Wan-AI/Wan2.2-I2V-A14B",
		Input: &schemas.VideoGenerationInput{
			Prompt:         "Animate the reference",
			InputReference: &inputReference,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, converted)
	assert.Equal(t, "Wan-AI/Wan2.2-I2V-A14B", converted.Model)
	assert.Equal(t, "Animate the reference", converted.Prompt)
	assert.Equal(t, &inputReference, converted.Image)
}

func TestSiliconFlowVideoGenerationAndRetrieveRoundTrip(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, fmt.Sprintf("unexpected method: %s", r.Method), http.StatusMethodNotAllowed)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			http.Error(w, fmt.Sprintf("unexpected authorization: %s", got), http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/video/submit":
			var body SiliconFlowVideoSubmitRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, fmt.Sprintf("decode submit body: %v", err), http.StatusBadRequest)
				return
			}
			if body.Model != "Wan-AI/Wan2.2-T2V-A14B" || body.Prompt != "A paper boat on a river" {
				http.Error(w, fmt.Sprintf("unexpected submit body: %#v", body), http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"requestId":"request-123"}`))
		case "/v1/video/status":
			var body SiliconFlowVideoStatusRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, fmt.Sprintf("decode status body: %v", err), http.StatusBadRequest)
				return
			}
			if body.RequestID != "request-123" {
				http.Error(w, fmt.Sprintf("requestId was not stripped: %q", body.RequestID), http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"status":"Succeed","results":{"videos":[{"url":"https://cdn.example/video.mp4"}]}}`))
		default:
			http.Error(w, fmt.Sprintf("unexpected path: %s", r.URL.Path), http.StatusNotFound)
		}
	}))
	defer server.Close()

	provider := newSFFileBatchVideoTestProvider(server.URL)
	generated, bifrostErr := provider.VideoGeneration(sfFileBatchVideoTestContext(), sfFileBatchVideoTestKey(), &schemas.BifrostVideoGenerationRequest{
		Provider: schemas.SiliconFlow,
		Model:    "Wan-AI/Wan2.2-T2V-A14B",
		Input:    &schemas.VideoGenerationInput{Prompt: "A paper boat on a river"},
	})

	require.Nil(t, bifrostErr)
	require.NotNil(t, generated)
	assert.Equal(t, "request-123:siliconflow", generated.ID)
	assert.Equal(t, schemas.VideoStatusQueued, generated.Status)
	assert.Equal(t, "Wan-AI/Wan2.2-T2V-A14B", generated.Model)
	assert.Equal(t, "A paper boat on a river", generated.Prompt)
	assert.Positive(t, generated.CreatedAt)

	retrieved, bifrostErr := provider.VideoRetrieve(sfFileBatchVideoTestContext(), sfFileBatchVideoTestKey(), &schemas.BifrostVideoRetrieveRequest{
		Provider: schemas.SiliconFlow,
		ID:       generated.ID,
	})

	require.Nil(t, bifrostErr)
	require.NotNil(t, retrieved)
	assert.Equal(t, "request-123:siliconflow", retrieved.ID)
	assert.Equal(t, schemas.VideoStatusCompleted, retrieved.Status)
	require.Len(t, retrieved.Videos, 1)
	require.NotNil(t, retrieved.Videos[0].URL)
	assert.Equal(t, "https://cdn.example/video.mp4", *retrieved.Videos[0].URL)
	assert.Equal(t, schemas.VideoOutputTypeURL, retrieved.Videos[0].Type)
	assert.Equal(t, "video/mp4", retrieved.Videos[0].ContentType)
}

func TestSiliconFlowVideoDownloadUsesSignedURLWithoutAuthorization(t *testing.T) {
	const content = "deterministic-video-content"

	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, fmt.Sprintf("unexpected method: %s", r.Method), http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/signed/video.mp4" || r.URL.RawQuery != "token=abc123" {
			http.Error(w, fmt.Sprintf("unexpected signed URL: %s", r.URL.RequestURI()), http.StatusNotFound)
			return
		}
		if authorization := r.Header.Get("Authorization"); authorization != "" {
			http.Error(w, fmt.Sprintf("signed URL received authorization: %q", authorization), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte(content))
	}))
	defer downloadServer.Close()

	statusServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/video/status" {
			http.Error(w, fmt.Sprintf("unexpected status request: %s %s", r.Method, r.URL.Path), http.StatusNotFound)
			return
		}
		var body SiliconFlowVideoStatusRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, fmt.Sprintf("decode status body: %v", err), http.StatusBadRequest)
			return
		}
		if body.RequestID != "download-123" {
			http.Error(w, fmt.Sprintf("unexpected requestId: %q", body.RequestID), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"status":"Succeed","results":{"videos":[{"url":%q}]}}`, downloadServer.URL+"/signed/video.mp4?token=abc123")
	}))
	defer statusServer.Close()

	provider := newSFFileBatchVideoTestProvider(statusServer.URL)
	response, bifrostErr := provider.VideoDownload(sfFileBatchVideoTestContext(), sfFileBatchVideoTestKey(), &schemas.BifrostVideoDownloadRequest{
		Provider: schemas.SiliconFlow,
		ID:       "download-123:siliconflow",
	})

	require.Nil(t, bifrostErr)
	require.NotNil(t, response)
	assert.Equal(t, "download-123:siliconflow", response.VideoID)
	assert.Equal(t, "video/mp4", response.ContentType)
	assert.Equal(t, []byte(content), response.Content)
}
