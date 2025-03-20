package main

import (
	"context"
	"io"
	"os"
	"time"

	pb "codeberg.org/n30w/jasima/n-talk/chat"
	"codeberg.org/n30w/jasima/n-talk/llms"
	"codeberg.org/n30w/jasima/n-talk/memory"
	"github.com/charmbracelet/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type config struct {
	name       string
	peers      []string
	server     string
	model      llms.LLMProvider
	layer      int
	initialize string
}

type client struct {
	*config
	memory MemoryService
	llm    LLMService
	logger *log.Logger

	conn              grpc.BidiStreamingClient[pb.Message, pb.Message]
	chatServiceClient *grpc.ClientConn

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

	var err error

	c.chatServiceClient, err = grpc.NewClient(c.config.server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatalf("%v", err)
	}

	// The implementation of `Chat` is in the server code. This is where the
	// client establishes a first connection to the server.
	c.conn, err = pb.NewChatServiceClient(c.chatServiceClient).Chat(ctx)
	if err != nil {
		logger.Fatalf("could not create stream: %v", err)
	}

	// Initialize a connection.

	err = c.sendMessage(c.model.String())
	if err != nil {
		logger.Fatalf("Unable to establish server connection; failed to send message: %v", err)
	}

	logger.Debugf("Established connection to the server @ %s", c.server)

	return c, nil
}

func (c *client) SendInitialMessage(ctx context.Context) error {
	recipient := c.peers[0]

	if c.initialize != "" {

		c.logger.Infof("Initialization path is %s, sending initial message to %s", c.initialize, recipient)

		time.Sleep(1 * time.Second)

		file, err := os.Open(c.initialize)
		if err != nil {
			return err
		}

		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			return err
		}

		fileText := string(data)

		c.memory.Save(ctx, c.NewMessageTo(recipient, fileText))

		err = c.sendMessage(fileText)
		if err != nil {
			return err
		}

		c.logger.Info("Initial message sent successfully")
	}

	return nil
}

func (c *client) Teardown() {
	c.logger.Debug("Beginning teardown...")

	c.conn.CloseSend()
	c.chatServiceClient.Close()
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

	c.logger.Debug("Dispatching request to LLM...")

	t := timer(time.Now())

	result, err := c.llm.Request(ctx, a, prompt)
	if err != nil {
		return "", err
	}

	v := t()

	c.logger.Debugf("Response received from LLM, roundtrip %s", v.Truncate(1*time.Millisecond))

	return result, nil
}

func (c *client) SendMessage(errs chan<- error, response <-chan string) {
	for res := range response {

		c.logger.Debug("Sending message 📧")

		err := c.sendMessage(res)
		if err != nil {
			errs <- err
			return
		}

		c.logger.Debug("Message sent successfully")
	}
}

func (c *client) sendMessage(content string) error {
	err := c.conn.Send(&pb.Message{
		Sender:   c.name,
		Receiver: c.peers[0],
		Content:  content,
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *client) DispatchToLLM(ctx context.Context, errs chan<- error, response chan<- string, llmChan <-chan string) {
	for input := range llmChan {

		content := input
		receiver := c.peers[0]

		// First save the incoming message.

		c.logger.Debug("Saving message to memory...")

		c.memory.Save(ctx, c.NewMessageFrom(receiver, content))

		c.logger.Debug("Messaged saved to memory successfully")

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

		c.logger.Debug("Piping message to response channel...")

		response <- llmResponse
	}
}

func (c *client) ReceiveMessages(ctx context.Context, online bool, errs chan<- error, llmChan chan<- string) {
	for online {

		msg, err := c.conn.Recv()

		if err == io.EOF {
			online = false
		} else if err != nil {
			errs <- err
			return
		} else {

			c.logger.Debugf("Message received from %s", msg.Sender)

			// Send the data to the LLM.
			content := msg.Content

			if c.latch {
				c.memory.Save(ctx, c.NewMessageFrom(msg.Receiver, content))
				c.logger.Debug("Latch is TRUE. Saved to memory only!")
			} else {
				c.logger.Debug("Piping message to LLM service...")
				llmChan <- content
			}
		}
	}
}
