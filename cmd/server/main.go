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
	DefaultLogToFileToggle           = false
	DefaultLogToFilePath             = "./outputs/server_log_%s.log"
)

func main() {
	flagDebug := flag.Bool(
		"debug",
		DefaultDebugToggle,
		"debug mode, extra logging",
	)
	flagLogToFile := flag.Bool(
		"logToFile",
		DefaultLogToFileToggle,
		"also logs output to file",
	)
	flagSpecificationPath := flag.String(
		"specs",
		DefaultSpecificationResourcePath,
		"path to directory containing specifications",
	)
	flagExchanges := flag.Int(
		"exchanges",
		DefaultMaxExchanges,
		"total exchanges between agents per layer",
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
	specifications, err := NewLangSpecification(*flagSpecificationPath)
	if err != nil {
		logger.Fatal(err)
	}

	cs := NewConlangServer(
		"SERVER",
		logger,
		store,
		specifications,
		*flagExchanges,
	)

	cs.Run(errors)

	for e := range errors {
		if e != nil {
			logger.Fatal(e)
		}
	}
}
