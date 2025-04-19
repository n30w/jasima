package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"codeberg.org/n30w/jasima/n-talk/internal/memory"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/log"
)

func main() {
	var err error

	flagName := flag.String("name", "", "name of the agent")
	flagPeers := flag.String(
		"peers",
		"",
		"comma separated list of agent's peers",
	)
	flagServer := flag.String("server", "", "communication server")
	flagProvider := flag.Int("model", -1, "LLM model to use")
	flagDebug := flag.Bool("debug", false, "debug mode, extra logging")
	flagConfigPath := flag.String(
		"configFile",
		"./configs/default_agent.toml",
		"configuration file path",
	)
	flagTemperature := flag.Float64(
		"temperature",
		1.50,
		"float64 model temperature",
	)
	flagInitializePath := flag.String("initialize", "", "initial message path")
	flagLayer := flag.Int("layer", -1, "agent's functional layer")

	flag.Parse()

	logOptions := log.Options{
		ReportTimestamp: true,
	}

	if *flagDebug {
		logOptions.Level = log.DebugLevel
		logOptions.ReportCaller = true
	}

	logger := log.NewWithOptions(os.Stderr, logOptions)

	logger.Debug("DEBUG is set to TRUE")

	var userConf userConfig

	_, err = toml.DecodeFile(*flagConfigPath, &userConf)
	if err != nil {
		log.Fatal(err)
	}

	if *flagName != "" {
		userConf.Name = *flagName
	}

	if *flagPeers != "" {
		userConf.Peers[0] = *flagPeers
	}

	if *flagServer != "" {
		userConf.Network.Router = *flagServer
	}

	if *flagProvider != -1 {
		userConf.Model.Provider = *flagProvider
	}

	if *flagTemperature != 1.50 {
		userConf.Model.Temperature = *flagTemperature
	}

	if *flagInitializePath != "" {
		userConf.Model.Initialize = *flagInitializePath
	}

	if *flagLayer != -1 {
		userConf.Layer = int32(*flagLayer)
	}

	// system agents exist on layer 0.
	if userConf.Layer < 0 {
		logger.Fatal("`layer` parameter must be greater than or equal to 0")
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	defer stop()

	logger.Debug("Initializing memory storage")

	mem := memory.NewMemoryStore(0)

	client, err := newClient(ctx, userConf, mem, logger)
	if err != nil {
		logger.Fatal(err)
	}

	logger.Info(
		"Created new agent!",
		"name",
		client.Name,
		"model",
		client.llm,
		"layer",
		client.Layer,
	)

	// Send an initial message, if the initialization config parameter is set.

	err = client.SendInitialMessage(ctx)
	if err != nil {
		logger.Fatal(err)
	}

	// Set the status of the client to online.

	online := true

	responseChan := make(chan string)
	llmChan := make(chan string)
	errorChan := make(chan error)
	halt := make(chan os.Signal, 1)

	signal.Notify(halt, os.Interrupt, syscall.SIGTERM)

	// !!! UNLATCH FOR NOW !!!
	client.latch = false
	// !!! UNLATCH FOR NOW !!!

	// Send any message in the response channel.
	go client.SendMessages(errorChan, responseChan)

	// Wait for messages to come in and process them accordingly.
	go client.ReceiveMessages(ctx, online, errorChan, llmChan)

	// Watch for possible LLM dispatches.
	go client.DispatchToLLM(ctx, errorChan, responseChan, llmChan)

	select {
	case err = <-errorChan:
		logger.Fatalf("encountered error: %v", err)
	case <-ctx.Done():
		logger.Info("context done... goodnight.")
	case <-halt:
		logger.Info("halted, shutting down...")
	}

	client.Teardown()
}
