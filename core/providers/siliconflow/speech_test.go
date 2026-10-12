package siliconflow

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
)

func TestToSiliconFlowSpeechRequest(t *testing.T) {
	fishVoice := "fishaudio/fish-speech-1.5:alex"
	cosyVoice := "FunAudioLLM/CosyVoice2-0.5B:alex"
	references := []interface{}{
		map[string]interface{}{"audio": "data:audio/wav;base64,UklGRg=="},
	}

	tests := []struct {
		name      string
		request   *schemas.BifrostSpeechRequest
		stream    bool
		wantError string
		validate  func(*testing.T, *SiliconFlowSpeechRequest)
	}{
		{
			name:      "nil request has empty input",
			request:   nil,
			wantError: "speech input text is required",
		},
		{
			name: "blank input",
			request: &schemas.BifrostSpeechRequest{
				Model: "fishaudio/fish-speech-1.5",
				Input: &schemas.SpeechInput{Input: "   "},
			},
			wantError: "speech input text is required",
		},
		{
			name: "fish requires voice",
			request: &schemas.BifrostSpeechRequest{
				Model: "fishaudio/fish-speech-1.5",
				Input: &schemas.SpeechInput{Input: "hello"},
			},
			wantError: "requires a string voice",
		},
		{
			name: "fish rejects references",
			request: newSiliconFlowSpeechTestRequest(
				"fishaudio/fish-speech-1.5",
				"hello",
				&fishVoice,
				references,
			),
			wantError: "does not support voice references",
		},
		{
			name: "CosyVoice rejects voice and references together",
			request: newSiliconFlowSpeechTestRequest(
				"FunAudioLLM/CosyVoice2-0.5B",
				"hello",
				&cosyVoice,
				references,
			),
			wantError: "accepts either a string voice or references, not both",
		},
		{
			name: "CosyVoice rejects missing voice and references",
			request: newSiliconFlowSpeechTestRequest(
				"FunAudioLLM/CosyVoice2-0.5B",
				"hello",
				nil,
				nil,
			),
			wantError: "requires either a string voice or references",
		},
		{
			name: "CosyVoice accepts voice only",
			request: newSiliconFlowSpeechTestRequest(
				"FunAudioLLM/CosyVoice2-0.5B",
				"hello",
				&cosyVoice,
				nil,
			),
			validate: func(t *testing.T, converted *SiliconFlowSpeechRequest) {
				t.Helper()
				if converted.Voice == nil || *converted.Voice != cosyVoice {
					t.Fatalf("voice = %v, want %q", converted.Voice, cosyVoice)
				}
				if siliconFlowSpeechHasReferences(converted.ExtraParams) {
					t.Fatalf("unexpected references: %#v", converted.ExtraParams)
				}
			},
		},
		{
			name: "CosyVoice accepts references only",
			request: newSiliconFlowSpeechTestRequest(
				"FunAudioLLM/CosyVoice2-0.5B",
				"hello",
				nil,
				references,
			),
			validate: func(t *testing.T, converted *SiliconFlowSpeechRequest) {
				t.Helper()
				if converted.Voice != nil {
					t.Fatalf("voice = %v, want nil", converted.Voice)
				}
				if !siliconFlowSpeechHasReferences(converted.ExtraParams) {
					t.Fatalf("references were not preserved: %#v", converted.ExtraParams)
				}
			},
		},
		{
			name: "unknown model",
			request: newSiliconFlowSpeechTestRequest(
				"unknown/tts-model",
				"hello",
				&fishVoice,
				nil,
			),
			wantError: "not a supported SiliconFlow speech model",
		},
		{
			name: "stream flag is propagated",
			request: newSiliconFlowSpeechTestRequest(
				"fishaudio/fish-speech-1.5",
				"hello",
				&fishVoice,
				nil,
			),
			stream: true,
		},
		{
			name: "multi voice configuration is rejected",
			request: &schemas.BifrostSpeechRequest{
				Model: "FunAudioLLM/CosyVoice2-0.5B",
				Input: &schemas.SpeechInput{Input: "hello"},
				Params: &schemas.SpeechParameters{
					VoiceConfig: &schemas.SpeechVoiceInput{
						MultiVoiceConfig: []schemas.VoiceConfig{
							{Speaker: "speaker-1", Voice: "voice-1"},
						},
					},
				},
			},
			wantError: "does not support multi-voice configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converted, err := ToSiliconFlowSpeechRequest(tt.request, tt.stream)
			if tt.wantError != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got request %#v", tt.wantError, converted)
				}
				if !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("error %q does not contain %q", err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("ToSiliconFlowSpeechRequest returned error: %v", err)
			}
			if converted == nil {
				t.Fatal("expected converted request")
			}
			if converted.Stream != tt.stream {
				t.Fatalf("stream = %t, want %t", converted.Stream, tt.stream)
			}
			if converted.Model != tt.request.Model || converted.Input != tt.request.Input.Input {
				t.Fatalf("unexpected converted request: %#v", converted)
			}
			if tt.validate != nil {
				tt.validate(t, converted)
			}
		})
	}
}

func TestSiliconFlowSpeechStreamRawBinary(t *testing.T) {
	audio := []byte{0x00, 0x01, 0x02, 0x7f, 0x80, 0xfe, 0xff}
	voice := "FunAudioLLM/CosyVoice2-0.5B:alex"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/v1/audio/speech" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/v1/audio/speech")
		}
		if authorization := r.Header.Get("Authorization"); authorization != "Bearer test-api-key" {
			t.Errorf("Authorization = %q, want %q", authorization, "Bearer test-api-key")
		}

		var wireRequest SiliconFlowSpeechRequest
		if err := json.NewDecoder(r.Body).Decode(&wireRequest); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if wireRequest.Model != "FunAudioLLM/CosyVoice2-0.5B" {
			t.Errorf("model = %q", wireRequest.Model)
		}
		if wireRequest.Input != "stream this" {
			t.Errorf("input = %q", wireRequest.Input)
		}
		if wireRequest.Voice == nil || *wireRequest.Voice != voice {
			t.Errorf("voice = %v, want %q", wireRequest.Voice, voice)
		}
		if !wireRequest.Stream {
			t.Error("stream flag was false")
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(audio); err != nil {
			t.Errorf("write audio: %v", err)
		}
	}))
	defer server.Close()

	provider, err := NewSiliconFlowProvider(&schemas.ProviderConfig{
		NetworkConfig: schemas.NetworkConfig{
			BaseURL:                        server.URL,
			DefaultRequestTimeoutInSeconds: 5,
			StreamIdleTimeoutInSeconds:     5,
		},
	}, siliconFlowSpeechTestLogger{})
	if err != nil {
		t.Fatalf("NewSiliconFlowProvider returned error: %v", err)
	}

	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	defer ctx.Cancel()
	postHookRunner := func(
		_ *schemas.BifrostContext,
		result *schemas.BifrostResponse,
		bifrostErr *schemas.BifrostError,
	) (*schemas.BifrostResponse, *schemas.BifrostError) {
		return result, bifrostErr
	}

	stream, bifrostErr := provider.SpeechStream(
		ctx,
		postHookRunner,
		nil,
		schemas.Key{Value: *schemas.NewSecretVar("test-api-key")},
		newSiliconFlowSpeechTestRequest(
			"FunAudioLLM/CosyVoice2-0.5B",
			"stream this",
			&voice,
			nil,
		),
	)
	if bifrostErr != nil {
		t.Fatalf("SpeechStream returned error: %#v", bifrostErr)
	}
	if stream == nil {
		t.Fatal("SpeechStream returned a nil channel")
	}

	var receivedAudio []byte
	sawDelta := false
	sawDone := false
	for chunk := range stream {
		if chunk == nil {
			continue
		}
		if chunk.BifrostError != nil {
			t.Fatalf("received stream error: %#v", chunk.BifrostError)
		}
		response := chunk.BifrostSpeechStreamResponse
		if response == nil {
			continue
		}
		switch response.Type {
		case schemas.SpeechStreamResponseTypeDelta:
			sawDelta = true
			receivedAudio = append(receivedAudio, response.Audio...)
		case schemas.SpeechStreamResponseTypeDone:
			sawDone = true
			if len(response.Audio) != 0 {
				t.Fatalf("done chunk audio length = %d, want 0", len(response.Audio))
			}
		}
	}

	if !sawDelta {
		t.Fatal("expected at least one speech audio delta chunk")
	}
	if !sawDone {
		t.Fatal("expected a speech audio done chunk")
	}
	if !bytes.Equal(receivedAudio, audio) {
		t.Fatalf("received audio = %v, want %v", receivedAudio, audio)
	}
}

func newSiliconFlowSpeechTestRequest(
	model string,
	input string,
	voice *string,
	references interface{},
) *schemas.BifrostSpeechRequest {
	params := &schemas.SpeechParameters{}
	if voice != nil {
		params.VoiceConfig = &schemas.SpeechVoiceInput{Voice: voice}
	}
	if references != nil {
		params.ExtraParams = map[string]interface{}{"references": references}
	}
	return &schemas.BifrostSpeechRequest{
		Model:  model,
		Input:  &schemas.SpeechInput{Input: input},
		Params: params,
	}
}

type siliconFlowSpeechTestLogger struct{}

func (siliconFlowSpeechTestLogger) Debug(string, ...any)                   {}
func (siliconFlowSpeechTestLogger) Info(string, ...any)                    {}
func (siliconFlowSpeechTestLogger) Warn(string, ...any)                    {}
func (siliconFlowSpeechTestLogger) Error(string, ...any)                   {}
func (siliconFlowSpeechTestLogger) Fatal(string, ...any)                   {}
func (siliconFlowSpeechTestLogger) SetLevel(schemas.LogLevel)              {}
func (siliconFlowSpeechTestLogger) SetOutputType(schemas.LoggerOutputType) {}
func (siliconFlowSpeechTestLogger) LogHTTPRequest(schemas.LogLevel, string) schemas.LogEventBuilder {
	return schemas.NoopLogEvent
}
