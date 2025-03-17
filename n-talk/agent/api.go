package main

import (
	"context"
	"fmt"
	"time"

	pb "codeberg.org/n30w/jasima/n-talk/chat"
	"codeberg.org/n30w/jasima/n-talk/llms"
	"codeberg.org/n30w/jasima/n-talk/memory"
	"github.com/charmbracelet/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type config struct {
	name   string
	server string
	model  llms.LLMProvider
}

type client struct {
	memory MemoryService
	llm    LLMService
	logger *log.Logger
	*config
}

func NewClient(ctx context.Context, llm LLMService, memory MemoryService, cfg *config, logger *log.Logger) (*client, error) {
	c := &client{
		memory: memory,
		llm:    llm,
		config: cfg,
		logger: logger,
	}

	a, err := c.memory.Retrieve(context.Background(), c.name, 0)
	if err != nil {
		return nil, err
	}

	c.logger.Printf("In Memory: %v", a)

	return c, nil
}

func (c *client) newMessage(text string) memory.Message {
	return memory.Message{
		Text:       text,
		Timestamp:  time.Now(),
		InsertedBy: c.name,
	}
}

func (c *client) NewMessageFrom(sender string, text string) memory.Message {
	m := c.newMessage(text)

	m.Role = 0
	m.Sender = sender
	m.Receiver = c.name

	return m
}

func (c *client) NewMessageTo(recipient string, text string) memory.Message {
	m := c.newMessage(text)

	m.Role = 1
	m.Receiver = recipient
	m.Sender = c.name

	return m
}

func (c *client) Request(ctx context.Context, prompt string) (string, error) {

	a, err := c.memory.Retrieve(ctx, c.name, 0)
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
