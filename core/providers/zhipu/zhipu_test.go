package zhipu_test

import (
	"os"
	"strings"
	"testing"

	"github.com/maximhq/bifrost/core/internal/llmtests"
	"github.com/maximhq/bifrost/core/schemas"
)

func TestZhipu(t *testing.T) {
	t.Parallel()
	if strings.TrimSpace(os.Getenv("ZHIPU_API_KEY")) == "" {
		t.Skip("Skipping Zhipu tests because ZHIPU_API_KEY is not set")
	}

	client, ctx, cancel, err := llmtests.SetupTest()
	if err != nil {
		t.Fatalf("Error initializing test setup: %v", err)
	}
	defer cancel()
	defer client.Shutdown()

	testConfig := llmtests.ComprehensiveTestConfig{
		Provider:             schemas.Zhipu,
		ChatModel:            envOrDefault("ZHIPU_MODEL", "glm-5"),
		ReasoningModel:       envOrDefault("ZHIPU_REASONING_MODEL", envOrDefault("ZHIPU_MODEL", "glm-5")),
		EmbeddingModel:       envOrDefault("ZHIPU_EMBEDDING_MODEL", "embedding-3"),
		RerankModel:          envOrDefault("ZHIPU_RERANK_MODEL", "rerank"),
		ImageGenerationModel: envOrDefault("ZHIPU_IMAGE_MODEL", "cogview-4"),
		Scenarios: llmtests.TestScenarios{
			SimpleChat:            true,
			CompletionStream:      true,
			MultiTurnConversation: true,
			ToolCalls:             true,
			ToolCallsStreaming:    true,
			Reasoning:             true,
			Embedding:             true,
			Rerank:                true,
			ImageGeneration:       true,
			CountTokens:           true,
		},
		SkipEmptyToolSchemas: true,
	}

	t.Run("ZhipuTests", func(t *testing.T) {
		llmtests.RunAllComprehensiveTests(t, client, ctx, testConfig)
	})
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
