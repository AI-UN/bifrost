package guardrails

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/maximhq/bifrost/core/network"
	"github.com/maximhq/bifrost/core/schemas"
)

// presidioSegmentSeparator joins one request's text segments into the single Analyzer call.
// A span that crosses it belongs to no segment and is dropped.
const presidioSegmentSeparator = "\n\n"

// maxPresidioResponseBytes bounds what is read from the Analyzer, so a misbehaving instance
// cannot exhaust the gateway's memory.
const maxPresidioResponseBytes = 8 << 20

// presidioClient is shared by every profile and by the verify/entity helpers. It resolves the
// global proxy per request through network.DefaultTransport, which installs no SSRF policy: the
// Analyzer is usually an internal address, so private and loopback targets must stay reachable.
// It carries no timeout of its own; every call is bounded by the profile's deadline, derived
// from the request context. It never follows a redirect: the analysis body is the request's
// unredacted text, which a 307/308 would replay at whatever host the Analyzer names. A redirect
// is returned to the caller and rejected as a non-2xx answer, as are verify and entity lookups.
var presidioClient = sync.OnceValue(func() *http.Client {
	return &http.Client{
		Transport:     network.DefaultTransport(network.ClientPurposeAPI),
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
})

// presidioDetector calls a Presidio Analyzer. Redaction happens here in Go, from the spans it
// reports, so the Analyzer is only ever asked to analyze.
type presidioDetector struct {
	endpoint       string
	apiKey         string
	language       string
	scoreThreshold *float64
	entities       []string
}

type presidioAnalyzeRequest struct {
	Text           string   `json:"text"`
	Language       string   `json:"language,omitempty"`
	Entities       []string `json:"entities,omitempty"`
	ScoreThreshold *float64 `json:"score_threshold,omitempty"` // a pointer so an explicit 0 is sent
}

type presidioAnalyzeResult struct {
	EntityType string  `json:"entity_type"`
	Start      int     `json:"start"`
	End        int     `json:"end"`
	Score      float64 `json:"score"`
}

// presidioSpan is one segment's byte range in the joined text.
type presidioSpan struct {
	start int
	end   int
}

// newPresidioDetector builds a profile's Analyzer client.
func newPresidioDetector(cfg PresidioConfig) (detector, error) {
	if err := validateAnalyzerURL(cfg.AnalyzerURL); err != nil {
		return nil, err
	}
	language := cfg.Language
	if language == "" {
		language = "en"
	}
	return &presidioDetector{
		endpoint:       strings.TrimRight(cfg.AnalyzerURL, "/"),
		apiKey:         apiKeyValue(cfg.APIKey),
		language:       language,
		scoreThreshold: cfg.ScoreThreshold,
		entities:       cfg.Entities,
	}, nil
}

// Detect makes one /analyze call for the whole request and maps the reported spans back to the
// segments they came from.
func (d *presidioDetector) Detect(ctx context.Context, segments []string) ([]finding, error) {
	joined, spans := joinSegments(segments)
	if joined == "" {
		return nil, nil
	}
	payload, err := json.Marshal(presidioAnalyzeRequest{
		Text:           joined,
		Language:       d.language,
		Entities:       d.entities,
		ScoreThreshold: d.scoreThreshold,
	})
	if err != nil {
		return nil, fmt.Errorf("presidio request: %w", err)
	}
	resp, err := presidioCall(ctx, http.MethodPost, d.endpoint+"/analyze", d.apiKey, payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := presidioStatus(resp); err != nil {
		return nil, err
	}
	var results []presidioAnalyzeResult
	payload, err = io.ReadAll(io.LimitReader(resp.Body, maxPresidioResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("presidio analyzer response: %w", err)
	}
	// A working Analyzer answers with a results array. A `null`, an object, an error envelope
	// behind a 200 — none of them is a clean scan, and an enforcing profile must fail closed.
	if trimmed := bytes.TrimSpace(payload); len(trimmed) == 0 || trimmed[0] != '[' {
		return nil, fmt.Errorf("presidio analyzer response is not a results array")
	}
	if err := json.Unmarshal(payload, &results); err != nil {
		return nil, fmt.Errorf("presidio analyze response: %w", err)
	}
	found, err := mapPresidioResults(joined, spans, results)
	if err != nil {
		return nil, err
	}
	return found, nil
}

// joinSegments concatenates the segments and records each one's byte range.
func joinSegments(segments []string) (string, []presidioSpan) {
	if len(segments) == 0 {
		return "", nil
	}
	size := len(presidioSegmentSeparator) * (len(segments) - 1)
	for _, segment := range segments {
		size += len(segment)
	}
	var builder strings.Builder
	builder.Grow(size)
	spans := make([]presidioSpan, 0, len(segments))
	for i, segment := range segments {
		if i > 0 {
			builder.WriteString(presidioSegmentSeparator)
		}
		start := builder.Len()
		builder.WriteString(segment)
		spans = append(spans, presidioSpan{start: start, end: builder.Len()})
	}
	return builder.String(), spans
}

// mapPresidioResults converts every reported span into a finding on the segment that contains
// it. Presidio indexes text in code points (Python string indices), so offsets are converted to
// bytes before slicing. A span outside the text is a malformed answer and is returned as an
// error, so an enforcing profile fails closed instead of trusting a partial analysis; a span
// reaching into a separator or across two segments belongs to no segment and is dropped.
func mapPresidioResults(joined string, spans []presidioSpan, results []presidioAnalyzeResult) ([]finding, error) {
	if len(results) == 0 {
		return nil, nil
	}
	offsets := runeByteOffsets(joined)
	found := make([]finding, 0, len(results))
	for _, result := range results {
		if result.Start < 0 || result.Start >= result.End || result.End >= len(offsets) {
			return nil, fmt.Errorf("presidio analyzer reported the span %d-%d, outside the %d code points it was sent", result.Start, result.End, len(offsets)-1)
		}
		start, end := offsets[result.Start], offsets[result.End]
		for _, span := range spans {
			if start < span.start || end > span.end {
				continue
			}
			if literal := joined[start:end]; literal != "" {
				found = append(found, finding{entityType: result.EntityType, literal: literal})
			}
			break
		}
	}
	return found, nil
}

// runeByteOffsets returns the byte offset of every code-point boundary, plus the end of the
// string, so offsets[codePointIndex] is the byte offset of that code point.
func runeByteOffsets(s string) []int {
	offsets := make([]int, 0, utf8.RuneCountInString(s)+1)
	for i := range s {
		offsets = append(offsets, i)
	}
	return append(offsets, len(s))
}

// presidioCall performs one Analyzer request.
func presidioCall(ctx context.Context, method, endpoint, apiKey string, body []byte) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, fmt.Errorf("presidio analyzer request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if apiKey != "" {
		// Presidio's Analyzer has no authentication of its own, so a deployment that needs one
		// puts it behind a proxy that terminates `Authorization: Bearer`.
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := presidioClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("presidio analyzer request: %w", err)
	}
	return resp, nil
}

// presidioStatus turns a non-2xx answer into an error. The body is drained, never read: it can
// echo request text.
func presidioStatus(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		return nil
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
	return fmt.Errorf("presidio analyzer returned HTTP %d", resp.StatusCode)
}

// apiKeyValue resolves a profile's API key, which may come from the environment or a vault.
func apiKeyValue(key *schemas.SecretVar) string {
	if key == nil {
		return ""
	}
	return key.GetValue()
}

// VerifyProfile checks that a profile is usable: its config is valid and, for presidio, the
// Analyzer answers /health.
func VerifyProfile(ctx context.Context, p Profile) error {
	if err := ValidateProfile(p); err != nil {
		return err
	}
	if p.ProviderName != ProviderPresidio {
		return nil
	}
	var cfg PresidioConfig
	if err := decodeConfig(p.Config, &cfg); err != nil {
		return err
	}
	endpoint := strings.TrimRight(cfg.AnalyzerURL, "/") + "/health"
	resp, err := presidioCall(ctx, http.MethodGet, endpoint, apiKeyValue(cfg.APIKey), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return presidioStatus(resp)
}

// PresidioSupportedEntities asks the Analyzer which entities it can detect for a language, so an
// operator can pick them without reaching the Analyzer from a browser.
func PresidioSupportedEntities(ctx context.Context, cfg PresidioConfig) ([]string, error) {
	if err := validateAnalyzerURL(cfg.AnalyzerURL); err != nil {
		return nil, err
	}
	language := cfg.Language
	if language == "" {
		language = "en"
	}
	endpoint := strings.TrimRight(cfg.AnalyzerURL, "/") + "/supportedentities?language=" + url.QueryEscape(language)
	resp, err := presidioCall(ctx, http.MethodGet, endpoint, apiKeyValue(cfg.APIKey), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := presidioStatus(resp); err != nil {
		return nil, err
	}
	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxPresidioResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("presidio analyzer response: %w", err)
	}
	// The Analyzer answers with recognizer records; a few proxies and older builds answer with
	// bare entity names, so both shapes are accepted.
	var records []struct {
		EntityType string `json:"entity_type"`
	}
	var entities []string
	if err := json.Unmarshal(payload, &records); err == nil && len(records) > 0 {
		for _, record := range records {
			if record.EntityType != "" {
				entities = append(entities, record.EntityType)
			}
		}
	} else if err := json.Unmarshal(payload, &entities); err != nil {
		return nil, fmt.Errorf("presidio analyzer response is not a list of entities")
	}
	entities = dedupeStrings(entities)
	sort.Strings(entities)
	return entities, nil
}
