package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	pb "codeberg.org/n30w/jasima/n-talk/chat"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const ModelName = "gemini-2.0-flash"

// const Prompt = "I'm using the Google Gemini API. I'm trying to make sure that every time I send a query, the model remembers what we were talking about before. How do I do this? I'm using Go, not Python."

const Prompt = "Give me dinner ideas for one person."

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// apiKey := os.Getenv("GEMINI_API_KEY")

	// memory := NewMemoryStore()

	ctx := context.Background()

	// c, err := NewClient(ctx, apiKey, "gemini-2.0-flash", memory)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// res, err := c.Request(ctx, Prompt)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// fmt.Println(res)

	// err = c.RequestStream(ctx, Prompt)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewChatServiceClient(conn)
	stream, err := client.Chat(ctx)
	if err != nil {
		log.Fatalf("could not create stream: %v", err)
	}

	// Send messages
	go func() {
		messages := []*pb.Message{
			{Sender: "Client1", Content: "Hello from client 1"},
			{Sender: "Client1", Content: "How are you?"},
			{Sender: "Client1", Content: "Goodbye!"},
		}

		for _, msg := range messages {
			if err := stream.Send(msg); err != nil {
				log.Fatalf("failed to send message: %v", err)
			}
			fmt.Printf("Client sent: %v\n", msg)
			time.Sleep(1 * time.Second) // Simulate delay
		}
		stream.CloseSend()
	}()

	// Receive messages
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("failed to receive message: %v", err)
		}
		fmt.Printf("Client received: %v\n", msg)
	}
}
