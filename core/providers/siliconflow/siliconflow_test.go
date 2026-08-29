package siliconflow_test

import (
	"os"
	"strings"
	"testing"

	"github.com/maximhq/bifrost/core/internal/llmtests"
	"github.com/maximhq/bifrost/core/schemas"
)

func TestSiliconFlow(t *testing.T) {
	t.Parallel()
	if strings.TrimSpace(os.Getenv("SILICONFLOW_API_KEY")) == "" {
		t.Skip("Skipping SiliconFlow tests because SILICONFLOW_API_KEY is not set")
	}

	client, ctx, cancel, err := llmtests.SetupTest()
	if err != nil {
		t.Fatalf("Error initializing test setup: %v", err)
	}
	defer cancel()
	defer client.Shutdown()

	testConfig := llmtests.ComprehensiveTestConfig{
		Provider:             schemas.SiliconFlow,
		TextModel:            getEnvWithDefault("SILICONFLOW_TEXT_MODEL", "deepseek-ai/DeepSeek-V3.2"),
		ChatModel:            getEnvWithDefault("SILICONFLOW_CHAT_MODEL", "deepseek-ai/DeepSeek-V3.2"),
		VisionModel:          getEnvWithDefault("SILICONFLOW_VISION_MODEL", "Qwen/Qwen2.5-VL-72B-Instruct"),
		ReasoningModel:       getEnvWithDefault("SILICONFLOW_REASONING_MODEL", "deepseek-ai/DeepSeek-V3.2"),
		EmbeddingModel:       getEnvWithDefault("SILICONFLOW_EMBEDDING_MODEL", "Qwen/Qwen3-Embedding-0.6B"),
		RerankModel:          getEnvWithDefault("SILICONFLOW_RERANK_MODEL", "Qwen/Qwen3-Reranker-0.6B"),
		SpeechSynthesisModel: getEnvWithDefault("SILICONFLOW_SPEECH_MODEL", "FunAudioLLM/CosyVoice2-0.5B"),
		TranscriptionModel:   getEnvWithDefault("SILICONFLOW_TRANSCRIPTION_MODEL", "FunAudioLLM/SenseVoiceSmall"),
		ImageGenerationModel: getEnvWithDefault("SILICONFLOW_IMAGE_MODEL", "black-forest-labs/FLUX.1-schnell"),
		ImageEditModel:       getEnvWithDefault("SILICONFLOW_IMAGE_EDIT_MODEL", "Qwen/Qwen-Image-Edit"),
		VideoGenerationModel: getEnvWithDefault("SILICONFLOW_VIDEO_MODEL", "Wan-AI/Wan2.1-T2V-14B-720P-Turbo"),
		Scenarios: llmtests.TestScenarios{
			TextCompletion:         true,
			TextCompletionStream:   true,
			SimpleChat:             true,
			CompletionStream:       true,
			MultiTurnConversation:  true,
			ToolCalls:              true,
			ToolCallsStreaming:     true,
			End2EndToolCalling:     true,
			ImageURL:               true,
			ImageBase64:            true,
			MultipleImages:         true,
			SpeechSynthesis:        true,
			SpeechSynthesisStream:  true,
			Transcription:          true,
			Embedding:              true,
			Reasoning:              true,
			ListModels:             true,
			ImageGeneration:        true,
			ImageEdit:              true,
			VideoGeneration:        true,
			VideoRetrieve:          true,
			VideoDownload:          true,
			BatchCreate:            true,
			BatchList:              true,
			BatchRetrieve:          true,
			BatchCancel:            true,
			FileUpload:             true,
			FileList:               true,
			FileBatchInput:         true,
			PassThroughExtraParams: true,
			Rerank:                 true,
		},
	}

	t.Run("SiliconFlowTests", func(t *testing.T) {
		llmtests.RunAllComprehensiveTests(t, client, ctx, testConfig)
	})
}

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
