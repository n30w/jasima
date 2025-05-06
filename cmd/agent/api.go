package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"codeberg.org/n30w/jasima/pkg/agent"
	"codeberg.org/n30w/jasima/pkg/chat"
	"codeberg.org/n30w/jasima/pkg/llms"
	"codeberg.org/n30w/jasima/pkg/memory"
	"codeberg.org/n30w/jasima/pkg/utils"

	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type llmServices struct {
	gemini  *llms.GoogleGemini
	chatgpt *llms.OpenAIChatGPT
	ollama  *llms.Ollama
}

type client struct {
	*config
	memory MemoryService
	llm    LLMService
	logger *log.Logger

	conn       grpc.BidiStreamingClient[chat.Message, chat.Message]
	grpcClient *grpc.ClientConn

	// latch determines whether data that is received from the server is allowed
	// to be sent to the LLM service. If the latch is `true`, data will NOT be
	// sent to the LLM service, hence the data is "latched" onto the client. If
	// latch is `false`, data will be sent to the LLM service and returned.
	latch bool

	// sleepSeconds is the number of seconds to sleep between requests to an LLM
	// service. The number will differ based on model, but use the fastest time
	// for this value.
	sleepDuration time.Duration

	channels    *channels
	llmServices *llmServices
	online      bool
}

func newClient(
	ctx context.Context,
	userConf userConfig,
	mem MemoryService,
	logger *log.Logger,
	errs chan error,
) (*client, error) {
	var (
		err           error
		apiKey        string
		llm           LLMService
		sleepDuration time.Duration = 10
	)

	peerNames := make([]chat.Name, 0)
	for _, peer := range userConf.Peers {
		peerNames = append(peerNames, chat.Name(peer))
	}

	userConf.Model.Instructions = userConf.Model.
		Instructions + "Your name in this conversation is: " + userConf.Name

	cfg := &config{
		Name:          chat.Name(userConf.Name),
		Peers:         peerNames,
		Layer:         chat.SetLayer(userConf.Layer),
		ModelConfig:   userConf.Model,
		NetworkConfig: userConf.Network,
	}

	// Initialize the LLM service based on provider.

	err = godotenv.Load()
	if err != nil {
		return nil, err
	}

	ls := &llmServices{}

	switch cfg.ModelConfig.Provider {
	case llms.ProviderGoogleGemini_2_0_Flash:
		fallthrough
	case llms.ProviderGoogleGemini_2_5_Flash:
		apiKey = os.Getenv("GEMINI_API_KEY")
		ls.gemini, err = llms.NewGoogleGemini(
			apiKey,
			cfg.ModelConfig,
			logger,
		)
		if err != nil {
			return nil, err
		}

		llm = ls.gemini

	case llms.ProviderChatGPT:
		if cfg.ModelConfig.Temperature > 1.0 {
			logger.Warnf(
				"GPT with a temperature of %2f"+
					"may cause unexpected results! Consider values below 1.0.",
				cfg.ModelConfig.Temperature,
			)
		}

		apiKey = os.Getenv("CHATGPT_API_KEY")
		ls.chatgpt, err = llms.NewOpenAIChatGPT(
			apiKey,
			cfg.ModelConfig,
			logger,
		)
		if err != nil {
			return nil, err
		}

		llm = ls.chatgpt

	case llms.ProviderDeepseek:
		apiKey = os.Getenv("DEEPSEEK_API_KEY")
		llm, err = llms.NewDeepseek(
			apiKey,
			cfg.ModelConfig,
			logger,
		)
	case llms.ProviderOllama:
		ls.ollama, err = llms.NewOllama(
			nil,
			cfg.ModelConfig,
			logger,
		)
		if err != nil {
			return nil, err
		}

		llm = ls.ollama

		sleepDuration = 2

	case llms.ProviderClaude:
		apiKey = os.Getenv("CLAUDE_API_KEY")
		llm, err = llms.NewClaude(
			apiKey,
			cfg.ModelConfig,
			logger,
		)
	default:
		err = errors.New("invalid LLM provider")
	}
	if err != nil {
		return nil, err
	}

	logger.Debugf("%s is online and ready to go", llm)

	// Initialize gRPC facilities.

	grpcClient, err := grpc.NewClient(
		cfg.NetworkConfig.Router,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	// The implementation of `Chat` is in the server code. This is where the
	// client establishes the initial connection to the server.
	conn, err := chat.NewChatServiceClient(grpcClient).Chat(ctx)
	if err != nil {
		return nil, err
	}

	ch := &channels{
		responses: make(chan memory.Message),
		llm:       make(chan memory.Message),
		errs:      errs,
	}

	c := &client{
		memory:     mem,
		llm:        llm,
		config:     cfg,
		logger:     logger,
		grpcClient: grpcClient,
		conn:       conn,

		// Initially set `latch` to `true` so that data will only be sent in
		// lockstep with server commands.
		latch: true,

		// sleepDuration is the number of seconds to wait between each request
		// and receive from an LLM.
		sleepDuration: sleepDuration,

		channels:    ch,
		llmServices: ls,
		online:      true,
	}

	// Initialize the connection to the server.

	return c, nil
}

// initConnection runs to establish an initial connection to the server.
func (c *client) initConnection() error {
	content := chat.Content(c.llm.String())

	msg := c.NewMessageTo(c.Peers[0], content)

	c.channels.responses <- msg

	c.logger.Debugf(
		"Established connection to the server @ %s",
		c.NetworkConfig.Router,
	)

	return nil
}

func (c *client) Teardown() {
	c.logger.Debug("Beginning teardown...")

	_ = c.conn.CloseSend()
	_ = c.grpcClient.Close()
}

func (c *client) newMessage(text chat.Content) memory.Message {
	return memory.Message{
		Text:       text,
		Timestamp:  time.Now(),
		InsertedBy: c.Name,
	}
}

func (c *client) NewMessageFrom(
	sender chat.Name,
	text chat.Content,
) memory.Message {
	m := c.newMessage(text)

	m.Role = memory.UserRole
	m.Sender = sender
	m.Receiver = c.Name
	m.Layer = c.Layer

	return m
}

func (c *client) NewMessageTo(
	recipient chat.Name,
	text chat.Content,
) memory.Message {
	m := c.newMessage(text)

	m.Role = memory.ModelRole
	m.Receiver = recipient
	m.Sender = c.Name
	m.Layer = c.Layer

	return m
}

func (c *client) request(ctx context.Context) (
	chat.Content,
	error,
) {
	a, err := c.memory.Retrieve(ctx, c.Name, 0)
	if err != nil {
		return "", err
	}

	c.logger.Debug("Dispatching request to LLM...")

	t := utils.Timer(time.Now())

	result, err := c.llm.Request(ctx, a)
	if err != nil {
		return "", err
	}

	c.logger.Debugf(
		"Response received from LLM, roundtrip %s",
		t().Truncate(1*time.Millisecond),
	)

	return chat.Content(result), nil
}

func (c *client) SendMessages() {
	for res := range c.channels.responses {

		c.logger.Debug("Sending message 📧")

		err := c.sendMessage(res)
		if err != nil {
			c.channels.errs <- err
			return
		}

		c.logger.Debug("Message sent successfully")
	}
}

func (c *client) sendMessage(msg memory.Message) error {
	m := chat.NewPbMessage(c.Name, c.Peers[0], msg.Text, c.Layer)

	err := c.conn.Send(m)
	if err != nil {
		return err
	}

	return nil
}

func (c *client) DispatchToLLM(
	ctx context.Context,
	msg *memory.Message,
) {
	if c.latch {
		c.logger.Warn("Discarding response...", "latch", c.latch)
		return
	}

	err := c.memory.Save(ctx, c.NewMessageFrom(msg.Sender, msg.Text))
	if err != nil {
		c.channels.errs <- err
		return
	}

	c.logger.Debug("Messaged saved to memory successfully")

	time.Sleep(time.Second * c.sleepDuration)

	llmResponse, err := c.request(ctx)
	if err != nil {
		c.logger.Errorf(
			"Error requesting response from %s: %v",
			c.ModelConfig.Provider, err,
		)
		c.logger.Warn("Exiting dispatch procedure")
		return
	}

	if c.latch {
		c.logger.Warn("Discarding response...", "latch", c.latch)
		return
	}

	// Save the LLM's response to memory.

	newMsg := c.NewMessageTo(c.Name, llmResponse)

	err = c.memory.Save(ctx, newMsg)
	if err != nil {
		c.channels.errs <- err
		return
	}

	time.Sleep(time.Second * c.sleepDuration)

	// When data is received back from the query, fill the channel

	c.logger.Debug("Piping message to response channel...")

	c.channels.responses <- newMsg
}

// ReceiveMessages receives messages from the server.
func (c *client) ReceiveMessages(
	ctx context.Context,
) {
	defer ctx.Done()
	for {
		pbMsg, err := c.conn.Recv()
		ctx, cancel := context.WithCancelCause(ctx)

		if err != nil {
			var e error

			e = errors.Wrap(err, "Error receiving message")

			if err == io.EOF {
				e = errors.New("connection closed by server")
			}

			cancel(e)
			c.online = false
			c.channels.errs <- e
			continue
		}

		msg := memory.NewChatMessage(
			pbMsg.Sender, pbMsg.Receiver,
			pbMsg.Content, pbMsg.Layer, pbMsg.Command,
		)

		c.logger.Debugf("Message received from %s", msg.Sender)

		// Intercept commands from the server.

		statusMsg := fmt.Sprintf("Received %s", msg.Command)

		c.logger.Debugf("Received %s", msg.Command)

		switch msg.Command {
		case agent.AppendInstructions:
			cancel(errors.New(statusMsg))
			c.llm.AppendInstructions(msg.Text.String())
		case agent.SetInstructions:
			cancel(errors.New(statusMsg))
			c.llm.SetInstructions(msg.Text.String())
		case agent.ClearMemory:
			cancel(errors.New(statusMsg))
			err = c.memory.Clear()
			if err != nil {
				c.channels.errs <- err
				return
			}
		case agent.ResetInstructions:
			cancel(errors.New(statusMsg))
			c.llm.SetInstructions(c.ModelConfig.Instructions)
		case agent.SetResponseTypeToJson:
			// Change the response to structured JSON output.
		case agent.SetResponseTypeToText:
			// Change the response to text only. LLM will not use structured
			// JSON for output.
		case agent.Latch:
			if c.latch {
				c.logger.Debug("Already latched, doing nothing...")
				break
			}

			c.latch = true

			cancel(errors.New(statusMsg))

		case agent.RequestJsonDictionaryUpdate:

			go typedRequest[memory.DictionaryEntries](ctx, msg, c)
			ctx.Done()

		case agent.SendInitialMessage:

			cancel(errors.New(statusMsg))

			if c.latch {
				c.logger.Debug("Please UNLATCH before sending initial message")
				break
			}

			// Save the message body as the initial message.

			content := msg.Text
			recipient := c.Peers[0]

			m := c.NewMessageTo(recipient, content)

			err = c.memory.Save(ctx, m)
			if err != nil {
				c.channels.errs <- err
				return
			}

			c.channels.responses <- m

			c.logger.Info("Initial message sent successfully")

		case agent.Unlatch:
			if !c.latch {
				c.logger.Debug("Already unlatched, doing nothing...")
				break
			}

			c.latch = false

		default:
			// Empty message and NO_COMMAND, do nothing.
			if msg.Text == "" {
				cancel(errors.New(statusMsg))
			} else {
				// Send the data to the LLM.
				if c.latch {
					c.logger.Debug("Latch is TRUE. Only saving message...")
					err = c.memory.Save(
						ctx, c.NewMessageFrom(
							msg.Receiver,
							msg.Text,
						),
					)
					if err != nil {
						cancel(errors.New("failed to save to memory"))
						c.channels.errs <- err
						return
					}
				} else {
					c.logger.Debug("Dispatching message to LLM service...")
					go c.DispatchToLLM(ctx, msg)
				}
			}
		}

		err = context.Cause(ctx)
		if err != nil {
			c.logger.Warnf("Context cancelled: %s", err)
		}
	}
}

func (c *client) Run(ctx context.Context) {
	// Send any message in the response channel.
	go c.SendMessages()

	// Wait for messages to come in and process them accordingly.
	go c.ReceiveMessages(ctx)

	err := c.initConnection()
	if err != nil {
		c.channels.errs <- err
		return
	}
}
