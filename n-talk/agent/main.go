package main

import (
	"context"
	"flag"
	"io"
	"os"
	"os/signal"
	"syscall"

	pb "codeberg.org/n30w/jasima/n-talk/chat"
	"codeberg.org/n30w/jasima/n-talk/llms"
	"codeberg.org/n30w/jasima/n-talk/memory"
	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ModelConfig struct {
	Provider     int
	Instructions string
	Temperature  float64
	Initialize   string
}

type NetworkConfig struct {
	Router   string
	Database string
}

type ConfigFile struct {
	Name      string
	Recipient string
	Model     ModelConfig
	Network   NetworkConfig
}

func main() {
	var err error

	flagName := flag.String("name", "", "name of the agent")
	flagRecipient := flag.String("recipient", "", "name of the recipient agent")
	flagServer := flag.String("server", "", "communication server")
	flagProvider := flag.Int("model", -1, "LLM model to use")
	flagDebug := flag.Bool("debug", false, "debug mode, extra logging")
	configPath := flag.String("configFile", "./configs/a1.toml", "configuration file path")

	flag.Parse()

	var conf ConfigFile
	_, err = toml.DecodeFile(*configPath, &conf)
	if err != nil {
		log.Fatal(err)
	}

	name := conf.Name
	recipient := conf.Recipient
	router := conf.Network.Router
	provider := conf.Model.Provider

	if *flagName != "" {
		name = *flagName
	}

	if *flagRecipient != "" {
		recipient = *flagRecipient
	}

	if *flagServer != "" {
		router = *flagServer
	}

	if *flagProvider != -1 {
		provider = *flagProvider
		conf.Model.Provider = *flagProvider
	}

	ctx := context.Background()
	memory := memory.NewMemoryStore()

	llm, err := selectModel(ctx, conf.Model)
	if err != nil {
		log.Fatal(err)
	}

	logOptions := log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
	}

	if *flagDebug {
		logOptions.Level = log.DebugLevel
	}

	logger := log.NewWithOptions(os.Stderr, logOptions)

	logger.Debug("DEBUG is set to TRUE")

	cfg := &config{
		name:   name,
		server: router,
		model:  llms.LLMProvider(provider),
		peers:  []string{recipient},
	}

	client, err := NewClient(ctx, llm, memory, cfg, logger)
	if err != nil {
		log.Fatal(err)
	}

	connection, err := grpc.NewClient(client.config.server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("%v", err)
	}

	defer connection.Close()

	logger.Info("Created new agent!", "name", name, "server", router, "model", llm)

	c := pb.NewChatServiceClient(connection)

	// The implementation of `Chat` is in the server code. This is where the
	// client establishes a first connection to the server.
	conn, err := c.Chat(ctx)
	if err != nil {
		logger.Fatalf("could not create stream: %v", err)
	}

	client.conn = conn

	// Initialize a connection.

	err = conn.Send(&pb.Message{
		Sender:   name,
		Receiver: recipient,
		Content:  client.model.String(),
	})
	if err != nil {
		log.Fatalf("Unable to establish server connection; failed to send message: %v", err)
	}

	log.Info("Established connection to server")

	// Set the status of the client to online.

	online := true

	responseChan := make(chan string)
	llmChan := make(chan string)
	errorChan := make(chan error)
	stop := make(chan os.Signal, 1)

	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Send the initialize text.
	if conf.Model.Initialize != "" {
		file, err := os.Open(conf.Model.Initialize)
		if err != nil {
			log.Fatalf("Failed to open file: %v", err)
		}

		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			log.Fatalf("Failed to read file: %v", err)
		}

		fileText := string(data)

		client.memory.Save(ctx, client.NewMessageTo(recipient, fileText))
		err = conn.Send(&pb.Message{
			Sender:   name,
			Receiver: recipient,
			Content:  fileText,
		})
		if err != nil {
			log.Fatalf("Failed to send message: %v", err)
		}
	}

	// !!! UNLATCH FOR NOW !!!
	client.latch = false
	// !!! UNLATCH FOR NOW !!!

	// Send any message in the response channel.
	go client.SendMessage(conn, recipient, errorChan, responseChan)

	// Wait for messages to come in and process them accordingly.
	go client.ReceiveMessages(ctx, online, errorChan, llmChan)

	// Watch for possible LLM dispatches.
	go client.DispatchToLLM(ctx, errorChan, responseChan, llmChan)

	// GTFO on error.
	go func(stop chan<- os.Signal, e <-chan error) {
		for err := range e {
			if err != nil {
				logger.Fatal(err)
				stop <- os.Kill
			}
		}
	}(stop, errorChan)

	<-stop

	logger.Info("Shutting down. See you later.")
	conn.CloseSend()
}
