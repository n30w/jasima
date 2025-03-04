package main

import (
	"context"
	"fmt"

	pb "codeberg.org/n30w/jasima/n-talk/chat"
	"github.com/charmbracelet/log"
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
	logger *log.Logger
	*config
}

func NewClient(ctx context.Context, llm LLMService, memory Memory, cfg *config, logger *log.Logger) (*client, error) {
	c := &client{
		memory: memory,
		llm:    llm,
		config: cfg,
		logger: logger,
	}

	a, _ := c.memory.Retrieve(0)

	c.logger.Printf("In Memory: %v\n", a)

	return c, nil
}

func (c *client) Request(ctx context.Context, prompt string) (string, error) {

	a, err := c.memory.Retrieve(0)
	if err != nil {
		return "", err
	}

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
