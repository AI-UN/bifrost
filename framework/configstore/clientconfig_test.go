package configstore

import (
	"encoding/json"
	"testing"
)

func TestClientConfigUnmarshalConvertResponsesToChat(t *testing.T) {
	var enabled ClientConfig
	if err := json.Unmarshal([]byte(`{"compat":{"convert_responses_to_chat":true}}`), &enabled); err != nil {
		t.Fatalf("unmarshal enabled config: %v", err)
	}
	if !enabled.Compat.ConvertResponsesToChat {
		t.Fatal("expected convert_responses_to_chat to be enabled")
	}

	var omitted ClientConfig
	if err := json.Unmarshal([]byte(`{"compat":{}}`), &omitted); err != nil {
		t.Fatalf("unmarshal omitted config: %v", err)
	}
	if omitted.Compat.ConvertResponsesToChat {
		t.Fatal("expected convert_responses_to_chat to default to false when omitted")
	}
}
