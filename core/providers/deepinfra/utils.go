package deepinfra

// defaultBaseURL is the DeepInfra API host root. It deliberately excludes the /v1 prefix:
// the OpenAI-compatible surface, the native inference surface and the native
// text-to-speech surface all hang off the same host under different subtrees.
const defaultBaseURL = "https://api.deepinfra.com"

// OpenAI-compatible endpoint paths (https://api.deepinfra.com/v1/...).
const (
	pathModels              = "/v1/models"
	pathTextCompletions     = "/v1/completions"
	pathChatCompletions     = "/v1/chat/completions"
	pathEmbeddings          = "/v1/embeddings"
	pathAudioSpeech         = "/v1/audio/speech"
	pathAudioTranscriptions = "/v1/audio/transcriptions"
	pathImagesGenerations   = "/v1/images/generations"
	pathImagesEdits         = "/v1/images/edits"
	pathImagesVariations    = "/v1/images/variations"
)

// Native endpoint paths. These have no OpenAI-shaped equivalent on DeepInfra.
const (
	// pathInferencePrefix is joined with a model name to form /v1/inference/{model}.
	// It serves rerank, zero-shot classification and raw-prompt generation.
	pathInferencePrefix = "/v1/inference/"
	// pathTextToSpeechPrefix is joined with a voice id; the streaming variant appends "/stream".
	// This is the only DeepInfra endpoint that emits audio incrementally.
	pathTextToSpeechPrefix = "/v1/text-to-speech/"
	pathVideos             = "/v1/videos"
)

// defaultSpeechStreamVoice is DeepInfra's default text-to-speech voice id. The streaming
// endpoint takes the voice in the path, so a request that omits one still needs a value.
const defaultSpeechStreamVoice = "af_bella"

// speechStreamChunkSize is the read buffer for native raw-audio streaming, matching the
// size used by the other raw-audio provider in the tree.
const speechStreamChunkSize = 4096

// supportedTTSOutputFormats is DeepInfra's TtsResponseFormat enum. Anything outside it is
// dropped rather than forwarded, so an OpenAI-only format never fails the request upstream.
var supportedTTSOutputFormats = map[string]struct{}{
	"mp3":  {},
	"opus": {},
	"flac": {},
	"wav":  {},
	"pcm":  {},
}

// normalizeTTSOutputFormat maps a Bifrost response format onto DeepInfra's supported set,
// returning false when the caller did not ask for a representable format.
func normalizeTTSOutputFormat(format string) (string, bool) {
	if format == "" {
		return "", false
	}
	if _, ok := supportedTTSOutputFormats[format]; ok {
		return format, true
	}
	return "", false
}
