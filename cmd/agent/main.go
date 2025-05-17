package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/log"

	"codeberg.org/n30w/jasima/pkg/llms"
	"codeberg.org/n30w/jasima/pkg/memory"
)

func main() {
	var (
		flagName = flag.String(
			"name",
			DefaultAgentName,
			"name of the agent",
		)
		flagPeers = flag.String(
			"peers",
			DefaultPeers,
			"comma separated list of agent's peers",
		)
		flagServer = flag.String(
			"server",
			DefaultServerAddress,
			"main communication server and routing service",
		)
		flagProvider = flag.Int(
			"model",
			DefaultModel,
			"LLM service provider model to use",
		)
		flagDebug = flag.Bool(
			"debug",
			DefaultDebugToggle,
			"debug mode, extra logging",
		)
		flagConfigPath = flag.String(
			"configFile",
			DefaultAgentConfigPath,
			"configuration file path",
		)
		flagTemperature = flag.Float64(
			"temperature",
			DefaultTemperatureFloat,
			"float64 model temperature",
		)
		flagLayer = flag.Int(
			"layer",
			DefaultLayer,
			"agent's functional layer",
		)
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

	_, err := toml.DecodeFile(*flagConfigPath, &userConf)
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
		userConf.Model.Provider = llms.LLMProvider(*flagProvider)
	}

	if *flagTemperature != DefaultTemperatureFloat {
		userConf.Model.Temperature = *flagTemperature
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

	var (
		errs = make(chan error)
		halt = make(chan os.Signal, 1)
	)

	ms := &memoryServices{
		stm: memory.NewMemoryStore(0),
		ltm: memory.NewMemoryStore(0),
	}

	c, err := newClient(ctx, userConf, ms, logger, errs)
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

	signal.Notify(halt, os.Interrupt, syscall.SIGTERM)

	c.Run(ctx)

	go func() {
		select {
		case err = <-errs:
			logger.Error(err)
		case sig := <-halt:
			logger.Warnf("Received %s, shutting down...", sig)
		}
		stop()
	}()

	<-ctx.Done()

	err = c.Teardown()
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nmi tawa!\n")
}
