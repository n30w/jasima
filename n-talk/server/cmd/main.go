package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"codeberg.org/n30w/jasima/n-talk/memory"
	"codeberg.org/n30w/jasima/n-talk/server"
	"github.com/charmbracelet/log"
)

func main() {
	flagDebug := flag.Bool("debug", false, "debug mode, extra logging")
	flagLogToFile := flag.Bool("logToFile", false, "also logs output to file")
	flagSpecificationPath := flag.String(
		"specs",
		"../resources/specifications",
		"path to directory containing specifications",
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
	)

	errors := make(chan error)

	if *flagLogToFile {
		logFilePath := fmt.Sprintf("../outputs/server_log_%s.log", time.Now().Format(time.RFC3339))
		f := logOutput(logger, logFilePath, errors)
		defer f()
	}

	memory := server.ServerMemory{
		MemoryService: memory.NewMemoryStore(0),
	}

	// Load and serialize specifications.
	specifications, err := server.NewLangSpecification(*flagSpecificationPath)
	if err != nil {
		logger.Fatal(err)
	}

	server := server.NewConlangServer("SERVER", logger, memory, specifications)

	go server.ListenAndServe(errors)

	for err := range errors {
		if err != nil {
			logger.Fatal(err)
		}
	}
}
