package main

import (
	"context"
	"os"

	"codeberg.org/n30w/jasima/n-talk/llms"
	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"
)

func selectModel(ctx context.Context, mc ModelConfig) (LLMService, error) {

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
		llm = llms.NewOllama("qwen2.5:32b", "http://localhost:11434/api/chat", mc.Instructions, mc.Temperature)
	default:
		log.Fatal("invalid model")
	}
	if err != nil {
		return nil, err
	}

	return llm, nil
}
