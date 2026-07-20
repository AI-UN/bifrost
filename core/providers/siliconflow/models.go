package siliconflow

import (
	"fmt"
	"net/url"
)

// buildSiliconFlowModelsURL appends SiliconFlow's documented list-models query
// parameters (`type`, `sub_type`) from extra params onto the models endpoint.
// Only string values are accepted; other extra keys are ignored.
func buildSiliconFlowModelsURL(baseURL, path string, extraParams map[string]interface{}) (string, error) {
	fullURL := baseURL + path
	if len(extraParams) == 0 {
		return fullURL, nil
	}

	values := url.Values{}
	for _, key := range []string{"type", "sub_type"} {
		raw, ok := extraParams[key]
		if !ok || raw == nil {
			continue
		}
		str, ok := raw.(string)
		if !ok {
			return "", fmt.Errorf("list models extra param %q must be a string, got %T", key, raw)
		}
		if str != "" {
			values.Set(key, str)
		}
	}

	if encoded := values.Encode(); encoded != "" {
		fullURL += "?" + encoded
	}
	return fullURL, nil
}
