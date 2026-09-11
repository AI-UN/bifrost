package zai_test

import (
	"os"
	"strings"
	"testing"

	"github.com/maximhq/bifrost/core/internal/llmtests"
	"github.com/maximhq/bifrost/core/schemas"
)

func TestZAI(t *testing.T) {
	t.Parallel()
	if strings.TrimSpace(os.Getenv("ZAI_API_KEY")) == "" {
		t.Skip("Skipping Z.AI tests because ZAI_API_KEY is not set")
	}

	client, ctx, cancel, err := llmtests.SetupTest()
	if err != nil {
		t.Fatalf("Error initializing test setup: %v", err)
	}
	defer cancel()
	defer client.Shutdown()

	testConfig := llmtests.ComprehensiveTestConfig{
		Provider:             schemas.ZAI,
		ChatModel:            envOrDefault("ZAI_MODEL", "glm-5"),
		ReasoningModel:       envOrDefault("ZAI_REASONING_MODEL", envOrDefault("ZAI_MODEL", "glm-5")),
		ImageGenerationModel: envOrDefault("ZAI_IMAGE_MODEL", "glm-image"),
		Scenarios: llmtests.TestScenarios{
			SimpleChat:            true,
			CompletionStream:      true,
			MultiTurnConversation: true,
			ToolCalls:             true,
			ToolCallsStreaming:    true,
			Reasoning:             true,
			ImageGeneration:       true,
			CountTokens:           true,
		},
		SkipEmptyToolSchemas: true,
	}

	t.Run("ZAITests", func(t *testing.T) {
		llmtests.RunAllComprehensiveTests(t, client, ctx, testConfig)
	})
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
