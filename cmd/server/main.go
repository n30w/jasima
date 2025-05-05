package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"codeberg.org/n30w/jasima/pkg/memory"
	"codeberg.org/n30w/jasima/pkg/utils"

	"github.com/charmbracelet/log"
)

const (
	DefaultSpecResourcePath   = "./resources/specifications"
	DefaultDictionaryJsonPath = "./resources/specifications/dictionary.json"
	DefaultSvgResourcePath    = "./resources/logography"
	DefaultLogToFilePath      = "./outputs/logs/server_log_%s.log"
	DefaultDebugToggle        = false
	DefaultMaxExchanges       = 25
	DefaultMaxGenerations     = 1
	DefaultLogToFileToggle    = false
	DefaultServerName         = "SERVER"
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
			"specPath",
			DefaultSpecResourcePath,
			"path to directory containing specifications",
		)
		flagDictionaryJsonPath = flag.String(
			"dictPath",
			DefaultDictionaryJsonPath,
			"path to initial json dictionary",
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
		flagSvgPath = flag.String(
			"svgPath",
			DefaultSvgResourcePath,
			"path to svg files of the Toki Pona logography",
		)
		flagServerName = flag.String(
			"name",
			DefaultServerName,
			"server name",
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
		"Initializing with these options",
		"debug",
		*flagDebug,
		"logToFile",
		*flagLogToFile,
		"specs",
		*flagSpecificationPath,
		"exchanges",
		*flagExchanges,
		"generations",
		*flagGenerations,
		"name",
		*flagServerName,
	)

	cfg := &config{
		name:         *flagServerName,
		debugEnabled: *flagDebug,
		files: filePathConfig{
			specifications: *flagSpecificationPath,
			logography:     *flagSvgPath,
			dictionary:     *flagDictionaryJsonPath,
		},
		procedures: procedureConfig{
			maxExchanges:   *flagExchanges,
			maxGenerations: *flagGenerations,
		},
	}

	errs := make(chan error)

	if *flagLogToFile {
		logFilePath := fmt.Sprintf(
			DefaultLogToFilePath,
			time.Now().Format(time.RFC3339),
		)
		f := utils.LogOutput(logger, logFilePath, errs)
		defer f()
	}

	store := memory.NewMemoryStore(0)

	cs, err := NewConlangServer(
		cfg,
		logger,
		store,
	)
	if err != nil {
		logger.Fatal(err)
	}

	cs.Run(errs)

	for e := range errs {
		if e != nil {
			logger.Fatal(e)
		}
	}
}
