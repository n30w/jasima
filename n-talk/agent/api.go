package main

import (
	"context"
	"io"
	"time"

	pb "codeberg.org/n30w/jasima/n-talk/chat"
	"codeberg.org/n30w/jasima/n-talk/llms"
	"codeberg.org/n30w/jasima/n-talk/memory"
	"github.com/charmbracelet/log"
	"google.golang.org/grpc"
)

type config struct {
	name   string
	peers  []string
	server string
	model  llms.LLMProvider
}

type client struct {
	*config
	memory MemoryService
	llm    LLMService
	logger *log.Logger
	conn   grpc.BidiStreamingClient[pb.Message, pb.Message]

	// latch determines whether data that is received from the server is allowed
	// to be sent to the LLM service. If the latch is `true`, data will NOT be
	// sent to the LLM service, hence the data is "latched" onto the client. If
	// latch is `false`, data will be sent to the LLM service and returned.
	latch bool
}

func NewClient(ctx context.Context, llm LLMService, memory MemoryService, cfg *config, logger *log.Logger) (*client, error) {
	c := &client{
		memory: memory,
		llm:    llm,
		config: cfg,
		logger: logger,

		// Initially set `latch` to `true` so that data will only be sent in
		// lockstep with server commands.
		latch: true,
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

func (c *client) request(ctx context.Context, prompt string) (string, error) {
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

func (c *client) SendMessage(conn grpc.BidiStreamingClient[pb.Message, pb.Message], receiver string, errs chan<- error, response <-chan string) {
	for res := range response {
		err := conn.Send(&pb.Message{
			Sender:   c.name,
			Receiver: receiver,
			Content:  res,
		})
		if err != nil {
			errs <- err
			return
		}
	}
}

func (c *client) DispatchToLLM(ctx context.Context, errs chan<- error, response chan<- string, llmChan <-chan string) {
	for input := range llmChan {

		content := input
		receiver := c.peers[0]

		// First save the incoming message.

		c.memory.Save(ctx, c.NewMessageFrom(receiver, content))

		if c.model != llms.ProviderOllama {
			time.Sleep(time.Second * 18)
		}

		time.Sleep(time.Second * 2)

		llmResponse, err := c.request(ctx, content)
		if err != nil {
			errs <- err
			return
		}

		// Save the LLM's response to memory.

		c.memory.Save(ctx, c.NewMessageTo(c.name, llmResponse))

		// Sleep longer if the provider is NOT Ollama. Let's not hit rate
		// limits...

		if c.model != llms.ProviderOllama {
			time.Sleep(time.Second * 18)
		}

		time.Sleep(time.Second * 2)

		// When data is received back from the query, fill the channel

		response <- llmResponse
	}
}

func (c *client) ReceiveMessages(ctx context.Context, online bool, errs chan<- error, llmChan chan<- string) {
	for online {
		msg, err := c.conn.Recv()
		if err == io.EOF {
			// This exits the program when the connection is terminated by
			// the server.
			online = false
		} else if err != nil {
			errs <- err
			return
		} else {
			// Send the data to the LLM.
			content := msg.Content
			if c.latch {
				c.memory.Save(ctx, c.NewMessageFrom(msg.Receiver, content))
				c.logger.Debug("Latch is TRUE. Saved to memory only!")
			} else {
				llmChan <- content
				c.logger.Debug("Dispatched message to LLM")
			}
		}
	}
}
