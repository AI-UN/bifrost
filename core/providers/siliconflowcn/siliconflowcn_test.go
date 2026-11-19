package siliconflowcn_test

import (
	"os"
	"strings"
	"testing"

	"github.com/maximhq/bifrost/core/internal/llmtests"
	"github.com/maximhq/bifrost/core/schemas"
)

func TestSiliconFlowCN(t *testing.T) {
	t.Parallel()
	if strings.TrimSpace(os.Getenv("SILICONFLOWCN_API_KEY")) == "" {
		t.Skip("Skipping SiliconFlow CN tests because SILICONFLOWCN_API_KEY is not set")
	}

	client, ctx, cancel, err := llmtests.SetupTest()
	if err != nil {
		t.Fatalf("Error initializing test setup: %v", err)
	}
	defer cancel()
	defer client.Shutdown()

	// Model defaults follow the China service's own documentation; the two
	// regions publish different catalogues, so international ids are not
	// reused as defaults here.
	testConfig := llmtests.ComprehensiveTestConfig{
		Provider:             schemas.SiliconFlowCN,
		ChatModel:            getEnvWithDefault("SILICONFLOWCN_CHAT_MODEL", "deepseek-ai/DeepSeek-V3.2"),
		VisionModel:          getEnvWithDefault("SILICONFLOWCN_VISION_MODEL", "Qwen/Qwen2.5-VL-72B-Instruct"),
		ReasoningModel:       getEnvWithDefault("SILICONFLOWCN_REASONING_MODEL", "deepseek-ai/DeepSeek-V3.2"),
		EmbeddingModel:       getEnvWithDefault("SILICONFLOWCN_EMBEDDING_MODEL", "BAAI/bge-large-zh-v1.5"),
		RerankModel:          getEnvWithDefault("SILICONFLOWCN_RERANK_MODEL", "BAAI/bge-reranker-v2-m3"),
		SpeechSynthesisModel: getEnvWithDefault("SILICONFLOWCN_SPEECH_MODEL", "FunAudioLLM/CosyVoice2-0.5B"),
		TranscriptionModel:   getEnvWithDefault("SILICONFLOWCN_TRANSCRIPTION_MODEL", "FunAudioLLM/SenseVoiceSmall"),
		ImageGenerationModel: getEnvWithDefault("SILICONFLOWCN_IMAGE_MODEL", "Kwai-Kolors/Kolors"),
		ImageEditModel:       getEnvWithDefault("SILICONFLOWCN_IMAGE_EDIT_MODEL", "Qwen/Qwen-Image-Edit-2509"),
		VideoGenerationModel: getEnvWithDefault("SILICONFLOWCN_VIDEO_MODEL", "Wan-AI/Wan2.2-T2V-A14B"),
		Scenarios: llmtests.TestScenarios{
			// TextCompletion is deliberately absent: the China service does not
			// document /v1/completions and its FIM guide routes prefix/suffix
			// through /chat/completions instead. The provider still exposes the
			// operation, so callers who know it works can use it, but CI does
			// not assert on an undocumented upstream route.
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

	t.Run("SiliconFlowCNTests", func(t *testing.T) {
		llmtests.RunAllComprehensiveTests(t, client, ctx, testConfig)
	})
}

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
