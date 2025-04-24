package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"codeberg.org/n30w/jasima/memory"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/log"
)

const (
	DefaultAgentName = ""
	// DefaultAgentConfigPath is in relation to where the binary was run and
	// not the path where the binary exists.
	DefaultAgentConfigPath        = "./cmd/configs/default_agent.toml"
	DefaultServerAddress          = "localhost:50051"
	DefaultPeers                  = ""
	DefaultInitializationFilePath = ""
	DefaultTemperatureFloat       = 1.5
	DefaultModel                  = -1
	DefaultLayer                  = -1
	DefaultDebugToggle            = false
)

func main() {
	var err error

	flagName := flag.String(
		"name",
		DefaultAgentName,
		"name of the agent",
	)
	flagPeers := flag.String(
		"peers",
		DefaultPeers,
		"comma separated list of agent's peers",
	)
	flagServer := flag.String(
		"server",
		DefaultServerAddress,
		"main communication server and routing service",
	)
	flagProvider := flag.Int(
		"model",
		DefaultModel,
		"LLM service provider model to use",
	)
	flagDebug := flag.Bool(
		"debug",
		DefaultDebugToggle,
		"debug mode, extra logging",
	)
	flagConfigPath := flag.String(
		"configFile",
		DefaultAgentConfigPath,
		"configuration file path",
	)
	flagTemperature := flag.Float64(
		"temperature",
		DefaultTemperatureFloat,
		"float64 model temperature",
	)
	flagInitializePath := flag.String(
		"initialize",
		DefaultInitializationFilePath,
		"initial message file path",
	)
	flagLayer := flag.Int(
		"layer",
		DefaultLayer,
		"agent's functional layer",
	)

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
		logger.Error(err.Error())
		logger.Warnf("Failed to load agent config! Using defaults.")
	}

	if *flagName != DefaultAgentName {
		userConf.Name = *flagName
	}

	if *flagPeers != DefaultPeers {
		userConf.Peers[0] = *flagPeers
	}

	if *flagServer != DefaultServerAddress {
		userConf.Network.Router = *flagServer
	}

	if *flagProvider != DefaultModel {
		userConf.Model.Provider = *flagProvider
	}

	if *flagTemperature != DefaultTemperatureFloat {
		userConf.Model.Temperature = *flagTemperature
	}

	if *flagInitializePath != DefaultInitializationFilePath {
		userConf.Model.Initialize = *flagInitializePath
	}

	if *flagLayer != DefaultLayer {
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

	mem := memory.NewMemoryStore(0)

	logger.Debug("Initialized memory")

	c, err := newClient(ctx, userConf, mem, logger)
	if err != nil {
		logger.Fatal(err)
	}

	logger.Info(
		"Agent created!",
		"name",
		c.Name,
		"model",
		c.llm,
		"layer",
		c.Layer,
	)

	halt := make(chan os.Signal, 1)
	errs := make(chan error)

	signal.Notify(halt, os.Interrupt, syscall.SIGTERM)

	c.Run(ctx, errs)

	select {
	case err = <-errs:
		logger.Fatalf("encountered error: %v", err)
	case <-ctx.Done():
		logger.Info("context done... goodnight.")
	case <-halt:
		logger.Info("halted, shutting down...")
	}

	c.Teardown()
}
