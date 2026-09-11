package siliconflow

import (
	"encoding/json"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToSiliconFlowRerankRequest(t *testing.T) {
	topN := 2
	returnDocuments := true
	extraParams := map[string]interface{}{
		"model":     "override-model",
		"query":     "override-query",
		"documents": []string{"override-document"},
		"truncate":  "END",
		"priority":  3,
	}

	tests := []struct {
		name    string
		request *schemas.BifrostRerankRequest
		want    *SiliconFlowRerankRequest
		wantErr string
	}{
		{
			name:    "nil request",
			wantErr: "rerank request is nil",
		},
		{
			name: "blank query",
			request: &schemas.BifrostRerankRequest{
				Query:     " \t\n ",
				Documents: []schemas.RerankDocument{{Text: "document"}},
			},
			wantErr: "rerank query is required",
		},
		{
			name: "missing documents",
			request: &schemas.BifrostRerankRequest{
				Query: "query",
			},
			wantErr: "rerank documents are required",
		},
		{
			name: "projects documents and filters reserved extras",
			request: &schemas.BifrostRerankRequest{
				Model: "BAAI/bge-reranker-v2-m3",
				Query: "Which document is relevant?",
				Documents: []schemas.RerankDocument{
					{Text: "first", Meta: map[string]interface{}{"source": "one"}},
					{Text: "second"},
				},
				Params: &schemas.RerankParameters{
					TopN:            &topN,
					ReturnDocuments: &returnDocuments,
					ExtraParams:     extraParams,
				},
			},
			want: &SiliconFlowRerankRequest{
				Model:           "BAAI/bge-reranker-v2-m3",
				Query:           "Which document is relevant?",
				Documents:       []string{"first", "second"},
				TopN:            &topN,
				ReturnDocuments: &returnDocuments,
				ExtraParams: map[string]interface{}{
					"truncate": "END",
					"priority": 3,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToSiliconFlowRerankRequest(tt.request)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	assert.Equal(t, map[string]interface{}{
		"model":     "override-model",
		"query":     "override-query",
		"documents": []string{"override-document"},
		"truncate":  "END",
		"priority":  3,
	}, extraParams)
}

func TestSiliconFlowRerankResponse(t *testing.T) {
	tests := []struct {
		name            string
		fixture         string
		model           string
		wantID          string
		wantIndices     []int
		wantScores      []float64
		wantDocuments   []*schemas.RerankDocument
		wantPrompt      int
		wantCompletion  int
		wantTotalTokens int
	}{
		{
			name: "top-level tokens and deterministic sorting",
			fixture: `{
				"id":"rerank-top-level",
				"results":[
					{"index":3,"relevance_score":0.8,"document":{"text":"object document","id":"doc-3","source":"knowledge-base"}},
					{"index":2,"relevance_score":0.8,"document":"string document"},
					{"index":1,"relevance_score":0.9,"document":{"text":"highest score"}}
				],
				"tokens":{"input_tokens":11,"output_tokens":2}
			}`,
			model:       "BAAI/bge-reranker-v2-m3",
			wantID:      "rerank-top-level",
			wantIndices: []int{1, 2, 3},
			wantScores:  []float64{0.9, 0.8, 0.8},
			wantDocuments: []*schemas.RerankDocument{
				{Text: "highest score"},
				{Text: "string document"},
				{Text: "object document", ID: new("doc-3"), Meta: map[string]interface{}{"source": "knowledge-base"}},
			},
			wantPrompt:      11,
			wantCompletion:  2,
			wantTotalTokens: 13,
		},
		{
			name: "meta tokens",
			fixture: `{
				"id":"rerank-meta",
				"results":[
					{"index":0,"relevance_score":0.5,"document":"meta document"}
				],
				"meta":{"tokens":{"input_tokens":7,"output_tokens":1}}
			}`,
			model:       "Pro/BAAI/bge-reranker-v2-m3",
			wantID:      "rerank-meta",
			wantIndices: []int{0},
			wantScores:  []float64{0.5},
			wantDocuments: []*schemas.RerankDocument{
				{Text: "meta document"},
			},
			wantPrompt:      7,
			wantCompletion:  1,
			wantTotalTokens: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var upstream SiliconFlowRerankResponse
			require.NoError(t, json.Unmarshal([]byte(tt.fixture), &upstream))

			got := upstream.ToBifrostRerankResponse(tt.model)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantID, got.ID)
			assert.Equal(t, tt.model, got.Model)
			require.Len(t, got.Results, len(tt.wantIndices))

			for i := range got.Results {
				assert.Equal(t, tt.wantIndices[i], got.Results[i].Index)
				assert.Equal(t, tt.wantScores[i], got.Results[i].RelevanceScore)
				assert.Equal(t, tt.wantDocuments[i], got.Results[i].Document)
			}

			require.NotNil(t, got.Usage)
			assert.Equal(t, tt.wantPrompt, got.Usage.PromptTokens)
			assert.Equal(t, tt.wantCompletion, got.Usage.CompletionTokens)
			assert.Equal(t, tt.wantTotalTokens, got.Usage.TotalTokens)
		})
	}
}
