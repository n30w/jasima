package main

import (
	"context"
	"fmt"
	"log"

	pb "codeberg.org/n30w/jasima/n-talk/chat"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type config struct {
	name   string
	server string
}

type client struct {
	memory Memory
	llm    LLMService
	*config
}

func NewClient(ctx context.Context, llm LLMService, memory Memory, cfg *config) (*client, error) {
	c := &client{
		memory: memory,
		llm:    llm,
		config: cfg,
	}

	return c, nil
}

func (c *client) Request(ctx context.Context, prompt string) (string, error) {
	a := c.memory.All()

	result, err := c.llm.Request(ctx, a, prompt)
	if err != nil {
		return "", err
	}

	return result, nil
}

// Connect establishes a connection with the main server. It returns a gRPC
// streaming capable object.
func (c *client) Connect(ctx context.Context) (grpc.BidiStreamingClient[pb.Message, pb.Message], error) {
	fmt.Println("Making new client...")
	conn, err := grpc.NewClient(c.config.server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	defer conn.Close()

	fmt.Println("new client created")
	client := pb.NewChatServiceClient(conn)
	stream, err := client.Chat(ctx)
	if err != nil {
		log.Fatalf("could not create stream: %v", err)
	}
	fmt.Println("stream	client created")

	return stream, nil
}
