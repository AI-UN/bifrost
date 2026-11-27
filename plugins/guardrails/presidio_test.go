package guardrails

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/require"
)

// presidioSpanOf builds the code-point span Presidio would report for needle, which is what a
// Python-side analyzer returns: offsets count code points, not bytes.
func presidioSpanOf(t *testing.T, haystack, needle string) (int, int) {
	t.Helper()
	byteIndex := strings.Index(haystack, needle)
	require.GreaterOrEqual(t, byteIndex, 0, "needle %q is not in the joined text", needle)
	start := utf8.RuneCountInString(haystack[:byteIndex])
	return start, start + utf8.RuneCountInString(needle)
}

func TestMapPresidioResultsWithMultibyteText(t *testing.T) {
	segments := []string{"请联系 alex@example.com 谢谢", "😀 结尾 bob@example.com"}
	joined, spans := joinSegments(segments)

	const first = "alex@example.com"
	const second = "bob@example.com"
	firstStart, firstEnd := presidioSpanOf(t, joined, first)
	secondStart, secondEnd := presidioSpanOf(t, joined, second)

	found, err := mapPresidioResults(joined, spans, []presidioAnalyzeResult{
		{EntityType: "EMAIL_ADDRESS", Start: firstStart, End: firstEnd, Score: 0.9},
		{EntityType: "EMAIL_ADDRESS", Start: secondStart, End: secondEnd, Score: 0.9},
	})
	require.NoError(t, err)
	require.Equal(t, []finding{
		{entityType: "EMAIL_ADDRESS", literal: first},
		{entityType: "EMAIL_ADDRESS", literal: second},
	}, found)
}

func TestMapPresidioResultsWithEmojiOnly(t *testing.T) {
	// An emoji is one code point but four bytes: a byte-based conversion would slice mid-rune.
	segments := []string{"passport 😀😀ABC123"}
	joined, spans := joinSegments(segments)
	start, end := presidioSpanOf(t, joined, "😀😀ABC123")

	found, err := mapPresidioResults(joined, spans, []presidioAnalyzeResult{{EntityType: "ID", Start: start, End: end}})
	require.NoError(t, err)
	require.Equal(t, []finding{{entityType: "ID", literal: "😀😀ABC123"}}, found)
}

func TestMapPresidioResultsDropsCrossSegmentSpans(t *testing.T) {
	segments := []string{"first secret", "second secret"}
	joined, spans := joinSegments(segments)

	// One span covers the separator and both segments; neither segment alone contains it.
	start, end := presidioSpanOf(t, joined, "secret\n\nsecond")
	found, err := mapPresidioResults(joined, spans, []presidioAnalyzeResult{{EntityType: "X", Start: start, End: end}})
	require.NoError(t, err)
	require.Empty(t, found)

	// The separator alone belongs to no segment.
	separatorStart, separatorEnd := presidioSpanOf(t, joined, "\n\n")
	found, err = mapPresidioResults(joined, spans, []presidioAnalyzeResult{{EntityType: "X", Start: separatorStart, End: separatorEnd}})
	require.NoError(t, err)
	require.Empty(t, found)
}

func TestMapPresidioResultsRejectsMalformedSpans(t *testing.T) {
	// A span the Analyzer could not have derived from the text it was sent is a malformed
	// answer, not a clean scan: an enforcing profile must fail closed.
	joined, spans := joinSegments([]string{"abc"})
	for name, result := range map[string]presidioAnalyzeResult{
		"negative start":    {EntityType: "X", Start: -1, End: 2},
		"empty span":        {EntityType: "X", Start: 2, End: 2},
		"end past the text": {EntityType: "X", Start: 0, End: 999},
	} {
		t.Run(name, func(t *testing.T) {
			found, err := mapPresidioResults(joined, spans, []presidioAnalyzeResult{result})
			require.Error(t, err)
			require.Empty(t, found)
		})
	}

	found, err := mapPresidioResults(joined, spans, nil)
	require.NoError(t, err)
	require.Nil(t, found)
}

// analyzerServer starts a fake Analyzer that records the request it received.
func analyzerServer(t *testing.T, respond func(t *testing.T, w http.ResponseWriter, request presidioAnalyzeRequest)) (*httptest.Server, *capture) {
	t.Helper()
	captured := &capture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.mu.Lock()
		captured.method = r.Method
		captured.path = r.URL.Path
		captured.query = r.URL.RawQuery
		captured.authorization = r.Header.Get("Authorization")
		captured.mu.Unlock()

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		captured.mu.Lock()
		captured.body = body
		captured.mu.Unlock()

		var request presidioAnalyzeRequest
		require.NoError(t, json.Unmarshal(body, &request))
		respond(t, w, request)
	}))
	t.Cleanup(server.Close)
	return server, captured
}

type capture struct {
	mu            sync.Mutex
	method        string
	path          string
	query         string
	authorization string
	body          []byte
}

func (c *capture) analyzeRequest(t *testing.T) presidioAnalyzeRequest {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	var request presidioAnalyzeRequest
	require.NoError(t, json.Unmarshal(c.body, &request))
	return request
}

func TestPresidioDetectorSendsOneAnalyzeCallPerRequest(t *testing.T) {
	server, captured := analyzerServer(t, func(t *testing.T, w http.ResponseWriter, request presidioAnalyzeRequest) {
		start, end := presidioSpanOf(t, request.Text, "alex@example.com")
		require.NoError(t, json.NewEncoder(w).Encode([]presidioAnalyzeResult{
			{EntityType: "EMAIL_ADDRESS", Start: start, End: end, Score: 0.85},
		}))
	})

	zero := 0.0
	detector, err := newPresidioDetector(PresidioConfig{
		AnalyzerURL:    server.URL,
		APIKey:         schemas.NewSecretVar("proxy-token"),
		Language:       "zh",
		ScoreThreshold: &zero,
		Entities:       []string{"EMAIL_ADDRESS"},
	})
	require.NoError(t, err)

	found, err := detector.Detect(context.Background(), []string{"write to alex@example.com", "and nothing else"})
	require.NoError(t, err)
	require.Equal(t, []finding{{entityType: "EMAIL_ADDRESS", literal: "alex@example.com"}}, found)

	captured.mu.Lock()
	method, path, authorization, body := captured.method, captured.path, captured.authorization, captured.body
	captured.mu.Unlock()

	require.Equal(t, http.MethodPost, method)
	require.Equal(t, "/analyze", path)
	// Presidio has no authentication of its own; a proxy in front of it terminates this header.
	require.Equal(t, "Bearer proxy-token", authorization)
	// An explicit threshold of 0 must be sent, which omitempty on a value field would drop.
	require.Contains(t, string(body), `"score_threshold":0`)
	require.Contains(t, string(body), `"language":"zh"`)
	require.Contains(t, string(body), `"entities":["EMAIL_ADDRESS"]`)
	// All segments travel as one text, separated so the analyzer sees them as distinct blocks.
	require.Equal(t, "write to alex@example.com\n\nand nothing else", captured.analyzeRequest(t).Text)
}

func TestPresidioDetectorHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream said no"))
	}))
	t.Cleanup(server.Close)

	detector, err := newPresidioDetector(PresidioConfig{AnalyzerURL: server.URL})
	require.NoError(t, err)

	_, err = detector.Detect(context.Background(), []string{"text"})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "upstream said no", "an analyzer body can echo request text and must not reach the error")
}

func TestPresidioDetectorRejectsMalformedResponse(t *testing.T) {
	// A 200 is not a clean scan unless it carries a results array: an unhealthy Analyzer behind a
	// proxy can answer null or an error envelope, and an enforcing profile must fail closed.
	for name, body := range map[string]string{
		"null":       "null",
		"object":     `{"detail":"analyzer error"}`,
		"empty body": "",
		"not json":   "<html>ok</html>",
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			t.Cleanup(server.Close)

			detector, err := newPresidioDetector(PresidioConfig{AnalyzerURL: server.URL})
			require.NoError(t, err)
			_, err = detector.Detect(context.Background(), []string{"text"})
			require.Error(t, err)
		})
	}
}

func TestPresidioDetectorRejectsOutOfRangeSpan(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"entity_type":"EMAIL_ADDRESS","start":0,"end":999}]`))
	}))
	t.Cleanup(server.Close)

	detector, err := newPresidioDetector(PresidioConfig{AnalyzerURL: server.URL})
	require.NoError(t, err)
	_, err = detector.Detect(context.Background(), []string{"text"})
	require.Error(t, err)
}

func TestPresidioNeverFollowsRedirects(t *testing.T) {
	// The analyze body is the request's unredacted text, so a redirect must never replay it at
	// whatever host the Analyzer names.
	var reached bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(target.Close)
	redirecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(redirecting.Close)

	detector, err := newPresidioDetector(PresidioConfig{AnalyzerURL: redirecting.URL})
	require.NoError(t, err)
	_, err = detector.Detect(context.Background(), []string{"mail alex@example.com"})
	require.Error(t, err)

	config, err := json.Marshal(PresidioConfig{AnalyzerURL: redirecting.URL})
	require.NoError(t, err)
	require.Error(t, VerifyProfile(context.Background(), Profile{ProviderName: ProviderPresidio, Config: config}))
	_, err = PresidioSupportedEntities(context.Background(), PresidioConfig{AnalyzerURL: redirecting.URL})
	require.Error(t, err)
	require.False(t, reached, "a redirect must never reach another host")
}

func TestPresidioRedactsRequestEndToEnd(t *testing.T) {
	server, _ := analyzerServer(t, func(t *testing.T, w http.ResponseWriter, request presidioAnalyzeRequest) {
		start, end := presidioSpanOf(t, request.Text, "alex@example.com")
		require.NoError(t, json.NewEncoder(w).Encode([]presidioAnalyzeResult{
			{EntityType: "EMAIL_ADDRESS", Start: start, End: end, Score: 0.99},
		}))
	})

	config, err := json.Marshal(PresidioConfig{AnalyzerURL: server.URL, Action: ActionRedact, RedactionStrategy: StrategyMask})
	require.NoError(t, err)
	profile := Profile{ID: 1, ProviderName: ProviderPresidio, PolicyName: testPolicyName, Enabled: true, Config: config}
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})

	ctx := newTestContext()
	request := chatRequest("write to alex@example.com")
	require.NoError(t, plugin.PreRequestHook(ctx, request))
	require.Equal(t, "write to [EMAIL_ADDRESS:"+strings.Repeat("*", len("alex@example.com"))+"]", *request.ChatRequest.Input[0].Content.ContentStr)
}

func TestVerifyProfile(t *testing.T) {
	t.Run("healthy analyzer", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/health", r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()
		config, err := json.Marshal(PresidioConfig{AnalyzerURL: server.URL})
		require.NoError(t, err)
		require.NoError(t, VerifyProfile(context.Background(), Profile{ProviderName: ProviderPresidio, Config: config}))
	})

	t.Run("unhealthy analyzer", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()
		config, err := json.Marshal(PresidioConfig{AnalyzerURL: server.URL})
		require.NoError(t, err)
		require.Error(t, VerifyProfile(context.Background(), Profile{ProviderName: ProviderPresidio, Config: config}))
	})

	t.Run("in-process providers verify their config only", func(t *testing.T) {
		require.NoError(t, VerifyProfile(context.Background(), enabledRegexProfile(t, 1, emailPattern(ActionBlock))))
		require.Error(t, VerifyProfile(context.Background(), Profile{ProviderName: ProviderRegex, Config: json.RawMessage(`{"patterns":[{"pattern":"(["}]}`)}))
	})
}

func TestPresidioSupportedEntities(t *testing.T) {
	t.Run("recognizer records", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/supportedentities", r.URL.Path)
			require.Equal(t, "zh", r.URL.Query().Get("language"))
			_, _ = w.Write([]byte(`[{"entity_type":"EMAIL_ADDRESS","name":"Email"},{"entity_type":"PHONE_NUMBER","name":"Phone"},{"entity_type":"EMAIL_ADDRESS"}]`))
		}))
		defer server.Close()

		entities, err := PresidioSupportedEntities(context.Background(), PresidioConfig{AnalyzerURL: server.URL, Language: "zh"})
		require.NoError(t, err)
		require.Equal(t, []string{"EMAIL_ADDRESS", "PHONE_NUMBER"}, entities)
	})

	t.Run("bare names", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`["EMAIL_ADDRESS","PHONE_NUMBER"]`))
		}))
		defer server.Close()

		entities, err := PresidioSupportedEntities(context.Background(), PresidioConfig{AnalyzerURL: server.URL})
		require.NoError(t, err)
		require.Equal(t, []string{"EMAIL_ADDRESS", "PHONE_NUMBER"}, entities)
	})

	t.Run("unknown shape", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"unexpected":true}`))
		}))
		defer server.Close()

		_, err := PresidioSupportedEntities(context.Background(), PresidioConfig{AnalyzerURL: server.URL})
		require.Error(t, err)
	})
}
