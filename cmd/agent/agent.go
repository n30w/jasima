package main

import (
	"context"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"

	"codeberg.org/n30w/jasima/pkg/agent"
	"codeberg.org/n30w/jasima/pkg/chat"
	"codeberg.org/n30w/jasima/pkg/llms"
	"codeberg.org/n30w/jasima/pkg/memory"
	"codeberg.org/n30w/jasima/pkg/network"
	"codeberg.org/n30w/jasima/pkg/utils"
)

type channels struct {
	responses memory.MessageChannel
	llm       memory.MessageChannel
	inbound   chan *chat.Message
	errs      chan error
}

type client struct {
	*config
	*memoryServices

	llm    llmService
	logger *log.Logger

	// mc enables an agent to communicate with the server via a protocol
	// like gRPC. This client is only communicates with the `Message`
	// type from the `chat` package.
	mc messageService[chat.Message]

	// latch determines whether data that is received from the server is allowed
	// to be sent to the LLM service. If the latch is `true`, data will NOT be
	// sent to the LLM service, hence the data is "latched" onto the client. If
	// latch is `false`, data will be sent to the LLM service and returned.
	latch bool

	channels *channels

	// llmServices are the available LLM services the client may use to
	// initiate typed requests for JSON responses.
	llmServices *llmServices
	online      bool
}

func newClient(
	ctx context.Context,
	userConf userConfig,
	mem *memoryServices,
	logger *log.Logger,
	errs chan error,
) (*client, error) {
	var (
		err    error
		apiKey string
		llm    llmService
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

	chatInbound := make(chan *chat.Message)
	mc, err := network.NewChatClientService(ctx, userConf.Network.Router, chatInbound)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create client chat service")
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
		logger.Warnf(
			"Frequency Penalty and Presence Penalty are not provided for %s. Their values will be ignored.",
			cfg.ModelConfig.Provider,
		)

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

	ch := &channels{
		responses: make(chan memory.Message),
		llm:       make(chan memory.Message),
		inbound:   chatInbound,
		errs:      errs,
	}

	return &client{
		memoryServices: mem,
		llm:            llm,
		config:         cfg,
		logger:         logger,
		mc:             mc,
		// Initially set `latch` to `true` so that data will only be sent in
		// lockstep with server commands.
		latch:       true,
		channels:    ch,
		llmServices: ls,
		online:      true,
	}, nil
}

// action defines actions that the agent may take when receiving a message.
// ctx represents the current message context. prevCtxCancel is the cancel
// function for the context that preceded the current message context. ctxId is
// the context ID for the current message context. msg is the message that
// was received.
func (c *client) action(
	ctx context.Context,
	prevCtxCancel context.CancelFunc,
	ctxId int,
	msg *memory.Message,
) error {
	var err error

	// Before any action is executed, cancel the context that came before the
	// current context. This will cancel contexts used in go routines launched
	// by this function and may therefore cause a context canceled error.

	if prevCtxCancel != nil {
		prevCtxCancel()
	}

	switch msg.Command {
	case agent.AppendInstructions:
		c.llm.AppendInstructions(msg.Text.String())
	case agent.SetInstructions:
		c.llm.SetInstructions(msg.Text.String())
	case agent.ClearMemory:

		err = c.stm.Clear()
		if err != nil {
			return err
		}

	case agent.ResetInstructions:

		c.llm.SetInstructions(c.ModelConfig.Instructions)

	case agent.Latch:

		if !c.latch {
			c.latch = true
		}

	case agent.RequestJsonDictionaryUpdate:

		go typedRequest[memory.ResponseDictionaryEntries](ctx, msg, c)

	case agent.RequestLogogramIteration:

		go typedRequest[memory.ResponseLogogramIteration](ctx, msg, c)

	case agent.RequestLogogramCritique:

		go typedRequest[memory.ResponseLogogramCritique](ctx, msg, c)

	case agent.RequestDictionaryWordDetection:

		go typedRequest[memory.ResponseDictionaryWordsDetection](ctx, msg, c)

	case agent.SendInitialMessage:

		if c.latch {
			c.logger.Debug("Please UNLATCH before sending initial message")
			break
		}

		// Switch working context, use new memory.

		// Then ask the LLM to craft a response that will be used as the
		// initial message.

		// Hotswap the original memory back into place.

		// Save the message body as the initial message.

		content := msg.Text
		recipient := c.Peers[0]

		m := c.NewMessageTo(recipient, content)

		err = c.stm.Save(ctx, m)
		if err != nil {
			return err
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
		go c.DispatchToLLM(ctx)
	}

	return nil
}

// typedRequest dispatches a JSON request to a remote LLM service. For now, the
// response is serialized as a string so that it can be sent over gRPC. Ideally,
// this should be changed so that the gRPC channel accepts type `T` from the
// request return, however this will do for now.
func typedRequest[T any](ctx context.Context, msg *memory.Message, c *client) {
	select {
	case <-ctx.Done():
		c.logger.Warn("Exiting dispatch, context canceled")
		return
	default:
		err := c.stm.Save(
			ctx,
			c.NewMessageFrom(msg.Sender, msg.Text),
		)
		if err != nil {
			c.channels.errs <- err
			return
		}

		a, err := c.stm.Retrieve(ctx, c.Name, 0)
		if err != nil {
			c.channels.errs <- errors.Wrap(err, "stm retrieval failure")
			return
		}

		t := utils.Timer(time.Now())

		result, err := selectRequestType[T](ctx, a, c)
		switch {
		case errors.Is(err, context.Canceled):
			c.logger.Warn("LLM request context canceled")
			return
		case err != nil:
			c.channels.errs <- err
			return
		}

		c.logger.Debugf("Response took %s", t().Truncate(1*time.Millisecond))

		newMsg := c.NewMessageTo(c.Peers[0], chat.Content(result))

		err = c.stm.Save(ctx, newMsg)
		if err != nil {
			c.channels.errs <- errors.Wrap(err, "stm save failure")
			return
		}

		c.channels.responses <- newMsg
	}
}

// selectRequestType returns the result of a particular request given type `T`.
// `T` enforces the JSON schema of the request's body.
func selectRequestType[T any](
	ctx context.Context,
	messages []memory.Message, c *client,
) (string, error) {
	switch c.ModelConfig.Provider {
	case llms.ProviderGoogleGemini_2_0_Flash:
		fallthrough
	case llms.ProviderGoogleGemini_2_5_Flash:
		return llms.RequestTypedGoogleGemini[T](
			ctx,
			messages,
			c.llmServices.gemini,
			nil,
		)
	case llms.ProviderChatGPT:
		return llms.RequestTypedChatGPT[T](
			ctx,
			messages,
			c.llmServices.chatgpt,
			nil,
		)
	case llms.ProviderOllama:
		return llms.RequestTypedOllama[T](
			ctx,
			messages,
			c.llmServices.ollama,
			nil,
		)
	default:
		c.logger.Warnf(
			"JSON schema request for %s not supported, "+
				"using default request method",
			c.ModelConfig.Provider,
		)
		return c.llm.Request(ctx, messages, nil)
	}
}
