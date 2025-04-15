package main

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	pb "codeberg.org/n30w/jasima/n-talk/chat"
	"codeberg.org/n30w/jasima/n-talk/llms"
	"codeberg.org/n30w/jasima/n-talk/memory"
	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type config struct {
	*userConfig
}

type client struct {
	*config
	memory MemoryService
	llm    LLMService
	logger *log.Logger

	conn       grpc.BidiStreamingClient[pb.Message, pb.Message]
	grpcClient *grpc.ClientConn

	// latch determines whether data that is received from the server is allowed
	// to be sent to the LLM service. If the latch is `true`, data will NOT be
	// sent to the LLM service, hence the data is "latched" onto the client. If
	// latch is `false`, data will be sent to the LLM service and returned.
	latch bool

	// sleepSeconds is the number of seconds to sleep. This differs based on the
	// model chosen.
	sleepDuration time.Duration
}

func NewClient(ctx context.Context, cfg *config, memory MemoryService, logger *log.Logger) (*client, error) {
	var err error
	var apiKey string
	var llm LLMService
	var sleepDuration time.Duration = 18

	// Initialize the LLM service based on provider.

	err = godotenv.Load()
	if err != nil {
		return nil, err
	}

	switch llms.LLMProvider(cfg.Model.Provider) {
	case llms.ProviderGoogleGemini:
		apiKey = os.Getenv("GEMINI_API_KEY")
		llm, err = llms.NewGoogleGemini(ctx, apiKey, cfg.Model.Instructions, cfg.Model.Temperature)
	case llms.ProviderChatGPT:
		apiKey = os.Getenv("CHATGPT_API_KEY")
		llm, err = llms.NewOpenAIChatGPT(apiKey, cfg.Model.Instructions, cfg.Model.Temperature)
	case llms.ProviderDeepseek:
		panic("not implemented")
	case llms.ProviderOllama:
		llm, err = llms.NewOllama(nil, cfg.Model.Instructions, cfg.Model.Temperature)
		sleepDuration = 2
	default:
		err = errors.New("invalid LLM provider")
	}
	if err != nil {
		return nil, err
	}

	logger.Debugf("%s is online and ready to go", llm)

	// Initialize gRPC facilities.

	grpcClient, err := grpc.NewClient(cfg.Network.Router, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	// The implementation of `Chat` is in the server code. This is where the
	// client establishes the initial connection to the server.
	conn, err := pb.NewChatServiceClient(grpcClient).Chat(ctx)
	if err != nil {
		return nil, err
	}

	c := &client{
		memory:     memory,
		llm:        llm,
		config:     cfg,
		logger:     logger,
		grpcClient: grpcClient,
		conn:       conn,

		// Initially set `latch` to `true` so that data will only be sent in
		// lockstep with server commands.
		latch: true,

		sleepDuration: sleepDuration,
	}

	// Initialize the connection to the server.

	err = c.initConnection()
	if err != nil {
		return nil, err
	}

	return c, nil
}

// initConnection runs to establish an initial connection to the server.
func (c *client) initConnection() error {
	err := c.sendMessage(c.llm.String())
	if err != nil {
		return err
	}

	c.logger.Debugf("Established connection to the server @ %s", c.Network.Router)

	return nil
}

func (c *client) SendInitialMessage(ctx context.Context) error {
	recipient := c.Peers[0]

	if c.Model.Initialize != "" {

		c.logger.Infof("Initialization path is %s, sending initial message to %s", c.Model.Initialize, recipient)

		time.Sleep(1 * time.Second)

		file, err := os.Open(c.Model.Initialize)
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
	c.grpcClient.Close()
}

func (c *client) newMessage(text string) memory.Message {
	return memory.Message{
		Text:       text,
		Timestamp:  time.Now(),
		InsertedBy: c.Name,
	}
}

func (c *client) NewMessageFrom(sender string, text string) memory.Message {
	m := c.newMessage(text)

	m.Role = 0
	m.Sender = sender
	m.Receiver = c.Name

	return m
}

func (c *client) NewMessageTo(recipient string, text string) memory.Message {
	m := c.newMessage(text)

	m.Role = 1
	m.Receiver = recipient
	m.Sender = c.Name

	return m
}

func (c *client) request(ctx context.Context, prompt string) (string, error) {
	a, err := c.memory.Retrieve(ctx, c.Name, 0)
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
		Sender:   c.Name,
		Receiver: c.Peers[0],
		Content:  content,
		Layer:    c.Layer,
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *client) DispatchToLLM(ctx context.Context, errs chan<- error, response chan<- string, llmChan <-chan string) {
	for input := range llmChan {

		content := input
		receiver := c.Peers[0]

		// First save the incoming message.

		c.logger.Debug("Saving message to memory...")

		c.memory.Save(ctx, c.NewMessageFrom(receiver, content))

		c.logger.Debug("Messaged saved to memory successfully")

		time.Sleep(time.Second * c.sleepDuration)

		llmResponse, err := c.request(ctx, content)
		if err != nil {
			errs <- err
			return
		}

		// Save the LLM's response to memory.

		c.memory.Save(ctx, c.NewMessageTo(c.Name, llmResponse))

		time.Sleep(time.Second * c.sleepDuration)

		// When data is received back from the query, fill the channel

		c.logger.Debug("Piping message to response channel...")

		response <- llmResponse
	}
}

// ReceiveMessages receives messages from the server.
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
