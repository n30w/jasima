package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"codeberg.org/n30w/jasima/utils"

	"codeberg.org/n30w/jasima/memory"
	"github.com/charmbracelet/log"
)

const (
	// DefaultSpecificationResourcePath is in relation to where the command
	// was run, not where the binary exists.
	DefaultSpecificationResourcePath = "./resources/specifications"
	DefaultDebugToggle               = false
	DefaultMaxExchanges              = 25
	DefaultMaxGenerations            = 1
	DefaultLogToFileToggle           = false
	DefaultLogToFilePath             = "./outputs/logs/server_log_%s.log"
	DefaultServerName                = "SERVER"
)

func main() {
	var (
		flagDebug = flag.Bool(
			"debug",
			DefaultDebugToggle,
			"debug mode, extra logging",
		)
		flagLogToFile = flag.Bool(
			"logToFile",
			DefaultLogToFileToggle,
			"also logs output to file",
		)
		flagSpecificationPath = flag.String(
			"specs",
			DefaultSpecificationResourcePath,
			"path to directory containing specifications",
		)
		flagExchanges = flag.Int(
			"exchanges",
			DefaultMaxExchanges,
			"total exchanges between agents per layer",
		)
		flagGenerations = flag.Int(
			"generations",
			DefaultMaxGenerations,
			"maximum number of generations in evolution",
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

	logger.Info(
		"starting with these options",
		"debug",
		*flagDebug,
		"logToFile",
		*flagLogToFile,
		"specs",
		*flagSpecificationPath,
		"exchanges",
		*flagExchanges,
	)

	cfg := &config{
		name:         DefaultServerName,
		debugEnabled: !*flagDebug,
		procedures: procedureConfig{
			maxExchanges:          *flagExchanges,
			maxGenerations:        *flagGenerations,
			originalSpecification: nil,
			specifications:        nil,
		},
	}

	errors := make(chan error)

	if *flagLogToFile {
		logFilePath := fmt.Sprintf(
			DefaultLogToFilePath,
			time.Now().Format(time.RFC3339),
		)
		f := utils.LogOutput(logger, logFilePath, errors)
		defer f()
	}

	store := memory.NewMemoryStore(0)

	// Load and serialize specifications.

	specifications, err := newLangSpecification(*flagSpecificationPath)
	if err != nil {
		logger.Fatal(err)
	}

	// Load and serialize Toki Pona SVGs.

	cs := NewConlangServer(
		cfg,
		logger,
		store,
		specifications,
	)

	cs.Run(errors)

	for e := range errors {
		if e != nil {
			logger.Fatal(e)
		}
	}
}
