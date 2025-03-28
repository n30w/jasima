package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/log"
)

func main() {
	flagDebug := flag.Bool("debug", false, "debug mode, extra logging")
	flagLogToFile := flag.Bool("logToFile", false, "also logs output to file")

	flag.Parse()

	logOptions := log.Options{
		ReportTimestamp: true,
	}

	if *flagDebug {
		logOptions.Level = log.DebugLevel
		logOptions.ReportCaller = true
	}

	logger := log.NewWithOptions(os.Stderr, logOptions)

	logger.Info("starting with these options", "debug", *flagDebug, "logToFile", *flagLogToFile)

	errors := make(chan error)

	if *flagLogToFile {
		logFilePath := fmt.Sprintf("../outputs/server_log_%s.log", time.Now().Format(time.RFC3339))
		f := logOutput(logger, logFilePath, errors)
		defer f()
	}

	server := NewServer("SERVER", logger)

	go server.ListenAndServe(errors)

	for err := range errors {
		if err != nil {
			logger.Fatal(err)
		}
	}
}
