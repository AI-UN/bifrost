package siliconflow

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
)

func TestSiliconFlowImageEditField(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		wantField string
		wantError string
	}{
		{
			name:      "Qwen image edit",
			model:     "Qwen/Qwen-Image-Edit-2509",
			wantField: "image",
		},
		{
			name:      "Kontext dev",
			model:     "black-forest-labs/FLUX.1-Kontext-dev",
			wantField: "image",
		},
		{
			name:      "Kontext pro",
			model:     "black-forest-labs/FLUX.1-Kontext-pro",
			wantField: "input_image",
		},
		{
			name:      "Kontext max",
			model:     "black-forest-labs/FLUX.1-Kontext-max",
			wantField: "input_image",
		},
		{
			name:      "FLUX 1.1 pro",
			model:     "black-forest-labs/FLUX-1.1-pro",
			wantField: "image_prompt",
		},
		{
			name:      "FLUX 1.1 pro Ultra",
			model:     "black-forest-labs/FLUX-1.1-pro-Ultra",
			wantField: "image_prompt",
		},
		{
			name:      "unknown model",
			model:     "unknown/image-edit-model",
			wantError: "not a supported SiliconFlow image edit model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, err := siliconFlowImageEditField(tt.model)
			if tt.wantError != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tt.wantError)
				}
				if !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("error %q does not contain %q", err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("siliconFlowImageEditField returned error: %v", err)
			}
			if field != tt.wantField {
				t.Fatalf("field = %q, want %q", field, tt.wantField)
			}
		})
	}
}

func TestToSiliconFlowImageEditRequest(t *testing.T) {
	errorTests := []struct {
		name      string
		images    []schemas.ImageInput
		wantError string
	}{
		{
			name:      "no images",
			images:    []schemas.ImageInput{},
			wantError: "requires exactly one input image",
		},
		{
			name:      "empty image",
			images:    []schemas.ImageInput{{Image: []byte{}}},
			wantError: "requires exactly one input image",
		},
		{
			name: "multiple images",
			images: []schemas.ImageInput{
				{Image: []byte("first")},
				{Image: []byte("second")},
			},
			wantError: "supports exactly one input image, got 2",
		},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			converted, err := ToSiliconFlowImageEditRequest(&schemas.BifrostImageEditRequest{
				Model: "Qwen/Qwen-Image-Edit",
				Input: &schemas.ImageEditInput{
					Prompt: "edit this image",
					Images: tt.images,
				},
			})
			if err == nil {
				t.Fatalf("expected error containing %q, got request %#v", tt.wantError, converted)
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("error %q does not contain %q", err, tt.wantError)
			}
		})
	}

	t.Run("Qwen image is encoded as a base64 data URL", func(t *testing.T) {
		imageBytes := []byte("\x89PNG\r\n\x1a\n")
		converted, err := ToSiliconFlowImageEditRequest(&schemas.BifrostImageEditRequest{
			Model: "Qwen/Qwen-Image-Edit-2509",
			Input: &schemas.ImageEditInput{
				Prompt: "make it blue",
				Images: []schemas.ImageInput{{Image: imageBytes}},
			},
		})
		if err != nil {
			t.Fatalf("ToSiliconFlowImageEditRequest returned error: %v", err)
		}
		if converted.Image == nil {
			t.Fatal("expected image field to be populated")
		}
		wantDataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageBytes)
		if *converted.Image != wantDataURL {
			t.Fatalf("image = %q, want %q", *converted.Image, wantDataURL)
		}
		if converted.InputImage != nil || converted.ImagePrompt != nil {
			t.Fatalf("unexpected alternate image fields: input_image=%v image_prompt=%v", converted.InputImage, converted.ImagePrompt)
		}
	})
}

func TestToSiliconFlowImageGenerationRequest(t *testing.T) {
	tests := []struct {
		name          string
		size          *string
		n             *int
		wantImageSize *string
		wantBatchSize *int
	}{
		{
			name:          "maps size and n",
			size:          schemas.Ptr("1024x1024"),
			n:             schemas.Ptr(3),
			wantImageSize: schemas.Ptr("1024x1024"),
			wantBatchSize: schemas.Ptr(3),
		},
		{
			name:          "omits auto size",
			size:          schemas.Ptr(" AUTO "),
			wantImageSize: nil,
			wantBatchSize: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converted, err := ToSiliconFlowImageGenerationRequest(&schemas.BifrostImageGenerationRequest{
				Model: "black-forest-labs/FLUX.1-schnell",
				Input: &schemas.ImageGenerationInput{Prompt: "a mountain lake"},
				Params: &schemas.ImageGenerationParameters{
					Size: tt.size,
					N:    tt.n,
				},
			})
			if err != nil {
				t.Fatalf("ToSiliconFlowImageGenerationRequest returned error: %v", err)
			}
			assertOptionalString(t, "image_size", converted.ImageSize, tt.wantImageSize)
			assertOptionalInt(t, "batch_size", converted.BatchSize, tt.wantBatchSize)
		})
	}
}

func TestSiliconFlowImageResponseToBifrost(t *testing.T) {
	t.Run("converts URLs and seed", func(t *testing.T) {
		seed := 73
		response := &SiliconFlowImageResponse{
			Images: []SiliconFlowImageData{
				{URL: "https://images.example/first.png"},
				{URL: ""},
				{URL: "https://images.example/third.png"},
			},
			Seed: &seed,
		}

		converted, err := response.ToBifrostImageGenerationResponse(&SiliconFlowImageRequest{
			Model: "black-forest-labs/FLUX.1-schnell",
		})
		if err != nil {
			t.Fatalf("ToBifrostImageGenerationResponse returned error: %v", err)
		}
		if len(converted.Data) != 2 {
			t.Fatalf("data length = %d, want 2", len(converted.Data))
		}
		if converted.Data[0].URL != "https://images.example/first.png" || converted.Data[0].Index != 0 {
			t.Fatalf("unexpected first image data: %#v", converted.Data[0])
		}
		if converted.Data[1].URL != "https://images.example/third.png" || converted.Data[1].Index != 2 {
			t.Fatalf("unexpected second image data: %#v", converted.Data[1])
		}
		if converted.ImageGenerationResponseParameters == nil {
			t.Fatal("expected image generation response parameters")
		}
		if len(converted.Seeds) != 1 || converted.Seeds[0] != seed {
			t.Fatalf("seeds = %v, want [%d]", converted.Seeds, seed)
		}
	})

	tests := []struct {
		name   string
		images []SiliconFlowImageData
	}{
		{name: "empty image list", images: []SiliconFlowImageData{}},
		{name: "only empty URLs", images: []SiliconFlowImageData{{URL: ""}, {URL: ""}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converted, err := (&SiliconFlowImageResponse{Images: tt.images}).ToBifrostImageGenerationResponse(&SiliconFlowImageRequest{})
			if err == nil {
				t.Fatalf("expected no usable URL error, got response %#v", converted)
			}
			if !strings.Contains(err.Error(), "contained no image URLs") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func assertOptionalString(t *testing.T, name string, got, want *string) {
	t.Helper()
	if want == nil {
		if got != nil {
			t.Fatalf("%s = %q, want nil", name, *got)
		}
		return
	}
	if got == nil || *got != *want {
		t.Fatalf("%s = %v, want %q", name, got, *want)
	}
}

func assertOptionalInt(t *testing.T, name string, got, want *int) {
	t.Helper()
	if want == nil {
		if got != nil {
			t.Fatalf("%s = %d, want nil", name, *got)
		}
		return
	}
	if got == nil || *got != *want {
		t.Fatalf("%s = %v, want %d", name, got, *want)
	}
}
