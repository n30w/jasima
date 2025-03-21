package main

import (
	"flag"
	"os"

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
		f := logOutput(logger, errors)
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
