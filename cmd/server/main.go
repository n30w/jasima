package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"codeberg.org/n30w/jasima/pkg/memory"
	"codeberg.org/n30w/jasima/pkg/utils"

	"github.com/charmbracelet/log"
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
		flagDictExtractMethod = flag.Int(
			"dictionaryExtractMethod",
			DefaultDictionaryExtractionMethod,
			"dictionary extraction method",
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
		flagBroadcastTestData = flag.Bool(
			"broadcastTestData",
			DefaultBroadcastTestData,
			"broadcast test data for web events",
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

	cfg := &config{
		name:              *flagServerName,
		debugEnabled:      *flagDebug,
		broadcastTestData: *flagBroadcastTestData,
		files: filePathConfig{
			specifications: *flagSpecificationPath,
			logography:     *flagSvgPath,
			dictionary:     *flagDictionaryJsonPath,
		},
		procedures: procedureConfig{
			maxExchanges:                   *flagExchanges,
			maxGenerations:                 *flagGenerations,
			dictionaryWordExtractionMethod: dictExtractMethod(*flagDictExtractMethod),
		},
	}

	logger.Info(
		"Initializing with these options",
		"debug",
		cfg.debugEnabled,
		"logToFile",
		*flagLogToFile,
		"specs",
		cfg.files.specifications,
		"exchanges",
		cfg.procedures.maxExchanges,
		"generations",
		cfg.procedures.maxGenerations,
		"name",
		cfg.name,
		"broadcastTestData",
		cfg.broadcastTestData,
		"dictionaryExtractionMethod",
		cfg.procedures.dictionaryWordExtractionMethod,
	)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGTERM,
	)

	defer stop()

	var (
		errs = make(chan error)
		halt = make(chan os.Signal, 1)
		wg   = &sync.WaitGroup{}
	)

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
		errs,
	)
	if err != nil {
		logger.Fatal(err)
	}

	signal.Notify(
		halt,
		os.Interrupt,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGTERM,
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		cs.Run(ctx, wg)
	}()

	select {
	case err = <-errs:
		logger.Error(err)
		stop()
	case <-halt:
		stop()
	}

	<-ctx.Done()

	wg.Wait()

	close(errs)

	fmt.Printf("\n mi tawa! \n")
}
