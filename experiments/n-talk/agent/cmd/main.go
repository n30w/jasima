package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

const ModelName = "gemini-2.0-flash"
const Prompt = "Tell me about NYU Shanghai"

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	apiKey := os.Getenv("GEMINI_API_KEY")

	ctx := context.Background()

	c, err := NewClient(ctx, apiKey, "gemini-2.0-flash")
	if err != nil {
		log.Fatal(err)
	}

	res, err := c.Request(ctx, Prompt)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(res)
}
