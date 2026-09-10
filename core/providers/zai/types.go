package zai

import (
	"fmt"
	"strings"

	"github.com/maximhq/bifrost/core/providers/openai"
	providerUtils "github.com/maximhq/bifrost/core/providers/utils"
	"github.com/maximhq/bifrost/core/schemas"
)

const (
	defaultInternationalBaseURL = "https://api.z.ai/api/paas/v4"
	defaultChinaBaseURL         = "https://open.bigmodel.cn/api/paas/v4"

	pathChatCompletions  = "/chat/completions"
	pathImageGenerations = "/images/generations"
)

type serviceProfile struct {
	provider               schemas.ModelProvider
	defaultBaseURL         string
	imageModels            []string
	supportsImageWatermark bool
}

// supportsImageModel gates on the provider-stripped model id; the wire model is
// left untouched. Date-suffixed ids stay distinct because cogview-4-250304 is an
// international id while cogview-4 is a China-only id.
func (profile serviceProfile) supportsImageModel(model string) bool {
	_, bare := schemas.ParseModelString(model, profile.provider)
	bare = strings.ToLower(strings.TrimSpace(bare))
	for _, supported := range profile.imageModels {
		if bare == supported {
			return true
		}
	}
	return false
}

var (
	internationalProfile = serviceProfile{
		provider:       schemas.ZAI,
		defaultBaseURL: defaultInternationalBaseURL,
		imageModels:    []string{"glm-image", "cogview-4-250304"},
	}
	chinaProfile = serviceProfile{
		provider:               schemas.Zhipu,
		defaultBaseURL:         defaultChinaBaseURL,
		imageModels:            []string{"glm-image", "cogview-4", "cogview-3-flash"},
		supportsImageWatermark: true,
	}
)

type thinkingConfig struct {
	Type string `json:"type"`
}

// chatRequest keeps the shared OpenAI-shaped message and tool conversion while
// emitting the fields whose names or semantics differ on the Z.AI wire.
type chatRequest struct {
	openAI   *openai.OpenAIChatRequest
	thinking *thinkingConfig
	userID   *string
}

func (r *chatRequest) GetExtraParams() map[string]interface{} {
	if r == nil || r.openAI == nil {
		return nil
	}
	return r.openAI.GetExtraParams()
}

func (r *chatRequest) MarshalJSON() ([]byte, error) {
	if r == nil || r.openAI == nil {
		return []byte("null"), nil
	}

	body, err := r.openAI.MarshalJSON()
	if err != nil {
		return nil, err
	}
	if r.thinking != nil {
		body, err = providerUtils.SetJSONField(body, "thinking", r.thinking)
		if err != nil {
			return nil, fmt.Errorf("set thinking: %w", err)
		}
	}
	if r.userID != nil {
		body, err = providerUtils.SetJSONField(body, "user_id", *r.userID)
		if err != nil {
			return nil, fmt.Errorf("set user_id: %w", err)
		}
	}
	return body, nil
}
