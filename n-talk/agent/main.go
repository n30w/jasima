package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"codeberg.org/n30w/jasima/n-talk/memory"
	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/log"
)

type modelConfig struct {
	Provider     int
	Instructions string
	Temperature  float64
	Initialize   string
}

type networkConfig struct {
	Router   string
	Database string
}

type userConfig struct {
	Name    string
	Peers   []string
	Layer   int32
	Model   modelConfig
	Network networkConfig
}

func main() {
	var err error

	flagName := flag.String("name", "", "name of the agent")
	flagPeers := flag.String("peers", "", "comma separated list of agent's peers")
	flagServer := flag.String("server", "", "communication server")
	flagProvider := flag.Int("model", -1, "LLM model to use")
	flagDebug := flag.Bool("debug", false, "debug mode, extra logging")
	flagConfigPath := flag.String("configFile", "./configs/default_agent.toml", "configuration file path")
	flagTemperature := flag.Float64("temperature", 1.50, "float64 model temperature")
	flagInitializePath := flag.String("initialize", "", "initial message path")
	flagLayer := flag.Int("layer", -1, "agent's functional layer")

	flag.Parse()

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

	ctx := context.Background()
	memory := memory.NewMemoryStore(0)

	logOptions := log.Options{
		ReportTimestamp: true,
	}

	if *flagDebug {
		logOptions.Level = log.DebugLevel
		logOptions.ReportCaller = true
	}

	logger := log.NewWithOptions(os.Stderr, logOptions)

	logger.Debug("DEBUG is set to TRUE")

	cfg := &config{
		userConfig: &userConf,
	}

	client, err := NewClient(ctx, cfg, memory, logger)
	if err != nil {
		logger.Fatal(err)
	}

	logger.Info("Created new agent!", "name", client.Name, "model", client.llm, "layer", client.Layer)

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
	stop := make(chan os.Signal, 1)

	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// !!! UNLATCH FOR NOW !!!
	client.latch = false
	// !!! UNLATCH FOR NOW !!!

	// Send any message in the response channel.
	go client.SendMessage(errorChan, responseChan)

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

	client.Teardown()
}
