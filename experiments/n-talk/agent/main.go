package main

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"
)

const ModelName = "gemini-2.0-flash"

// const Prompt = "I'm using the Google Gemini API. I'm trying to make sure that every time I send a query, the model remembers what we were talking about before. How do I do this? I'm using Go, not Python."

const Prompt = "Give me dinner ideas for one person."

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	apiKey := os.Getenv("GEMINI_API_KEY")

	memory := NewMemoryStore()

	ctx := context.Background()

	c, err := NewClient(ctx, apiKey, "gemini-2.0-flash", memory)
	if err != nil {
		log.Fatal(err)
	}

	// res, err := c.Request(ctx, Prompt)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// fmt.Println(res)

	err = c.RequestStream(ctx, Prompt)
	if err != nil {
		log.Fatal(err)
	}
}
