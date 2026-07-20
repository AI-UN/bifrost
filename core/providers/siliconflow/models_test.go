package siliconflow

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSiliconFlowModelsURL(t *testing.T) {
	tests := []struct {
		name        string
		extraParams map[string]interface{}
		wantURL     string
		wantQuery   url.Values
		wantRaw     string
		wantErr     string
	}{
		{
			name:    "nil extras",
			wantURL: "https://api.siliconflow.cn/v1/models",
		},
		{
			name:        "empty extras",
			extraParams: map[string]interface{}{},
			wantURL:     "https://api.siliconflow.cn/v1/models",
		},
		{
			name: "type and subtype are escaped and ordered",
			extraParams: map[string]interface{}{
				"type":     "text/generation",
				"sub_type": "chat & reasoning",
			},
			wantURL: "https://api.siliconflow.cn/v1/models",
			wantQuery: url.Values{
				"type":     []string{"text/generation"},
				"sub_type": []string{"chat & reasoning"},
			},
			wantRaw: "sub_type=chat+%26+reasoning&type=text%2Fgeneration",
		},
		{
			name: "unrelated extras are ignored",
			extraParams: map[string]interface{}{
				"limit":  10,
				"cursor": "next page",
			},
			wantURL: "https://api.siliconflow.cn/v1/models",
		},
		{
			name: "non-string type is rejected",
			extraParams: map[string]interface{}{
				"type": 7,
			},
			wantErr: `list models extra param "type" must be a string, got int`,
		},
		{
			name: "non-string subtype is rejected",
			extraParams: map[string]interface{}{
				"sub_type": false,
			},
			wantErr: `list models extra param "sub_type" must be a string, got bool`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildSiliconFlowModelsURL("https://api.siliconflow.cn", "/v1/models", tt.extraParams)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)

			parsed, err := url.Parse(got)
			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, parsed.Scheme+"://"+parsed.Host+parsed.Path)
			assert.Equal(t, tt.wantQuery.Encode(), parsed.Query().Encode())
			assert.Equal(t, tt.wantRaw, parsed.RawQuery)
		})
	}
}
