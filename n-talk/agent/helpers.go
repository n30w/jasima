package main

import (
	"context"
	"errors"
	"os"
	"time"

	"codeberg.org/n30w/jasima/n-talk/llms"
	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"
)

func selectModel(ctx context.Context, mc ModelConfig, logger *log.Logger) (LLMService, error) {
	var llm LLMService
	var apiKey string

	err := godotenv.Load()
	if err != nil {
		return nil, err
	}

	switch llms.LLMProvider(mc.Provider) {
	case llms.ProviderGoogleGemini:
		apiKey = os.Getenv("GEMINI_API_KEY")
		llm, err = llms.NewGoogleGemini(ctx, apiKey, "gemini-2.0-flash", mc.Instructions, mc.Temperature)
	case llms.ProviderChatGPT:
		apiKey = os.Getenv("CHATGPT_API_KEY")
		llm, err = llms.NewOpenAIChatGPT("4o", apiKey, mc.Instructions, mc.Temperature)
	case llms.ProviderDeepseek:
		panic("not implemented")
	case llms.ProviderOllama:
		llm, err = llms.NewOllama("qwen2.5:32b", nil, mc.Instructions, mc.Temperature)
	default:
		return nil, errors.New("invalid model")
	}
	if err != nil {
		return nil, err
	}

	return llm, nil
}

func timer(start time.Time) func() time.Duration {
	return func() time.Duration {
		return time.Since(start)
	}
}
