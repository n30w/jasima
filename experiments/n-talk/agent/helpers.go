package main

import (
	"context"
	"os"

	"codeberg.org/n30w/jasima/n-talk/llms"
	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"
)

func selectModel(ctx context.Context, model int) (LLMService, error) {

	var llm LLMService
	var apiKey string

	err := godotenv.Load()
	if err != nil {
		return nil, err
	}

	switch llms.LLMProvider(model) {
	case llms.ProviderGoogleGemini:
		apiKey = os.Getenv("GEMINI_API_KEY")
		llm, err = llms.NewGoogleGemini(ctx, apiKey, "gemini-2.0-flash")
	case llms.ProviderChatGPT:
		apiKey = os.Getenv("CHATGPT_API_KEY")
		llm, err = llms.NewOpenAIChatGPT("4o", apiKey)
	case llms.ProviderDeepseek:
		panic("not implemented")
	case llms.ProviderOllama:
		llm = llms.NewOllama("qwen2.5:14b", "http://localhost:11434/api/chat")
	default:
		log.Fatal("invalid model")
	}
	if err != nil {
		return nil, err
	}

	return llm, nil
}
