package main

import (
	"context"

	"codeberg.org/n30w/jasima/n-talk/llms"
)

func selectModel(ctx context.Context, apiKey string, model int) (LLMService, error) {
	switch model {
	case 1:
		// Google Gemini
		googleGemini, err := llms.NewGoogleGemini(ctx, apiKey, "gemini-2.0-flash")

		return googleGemini, err

	case 2:
		// ChatGPT
		return nil, nil

	case 3:
		// Deepseek
		return nil, nil
	default:
		m, err := selectModel(ctx, apiKey, model)
		return m, err
	}
}
