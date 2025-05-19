package main

const (
	DefaultSpecResourcePath           = "./resources/specifications"
	DefaultDictionaryJsonPath         = "./resources/specifications/dictionary.json"
	DefaultSvgResourcePath            = "./resources/logography"
	DefaultLogToFilePath              = "./outputs/logs/server_log_%s.log"
	DefaultDebugToggle                = false
	DefaultBroadcastTestData          = false
	DefaultMaxExchanges               = 25
	DefaultMaxGenerations             = 1
	DefaultDictionaryExtractionMethod = 0
	DefaultLogToFileToggle            = false
	DefaultExportData                 = false
	DefaultServerName                 = "SERVER"
)

type procedureConfig struct {
	// maxExchanges represents the total exchanges allowed per layer
	// of evolution.
	maxExchanges int

	// maxGenerations represents the maximum number of generations to evolve.
	// When set to 0, the procedure evolves forever.
	maxGenerations int

	// dictionaryWordExtractionMethod defines which extraction method to use
	// for extracting dictionary words from a text. Two options exist:
	// `0` for regex-based and `1` for agent based.
	dictionaryWordExtractionMethod dictExtractMethod

	// exportGenerationData determines if a batch of jobs will export their
	// resulting data.
	exportData bool
}

type filePathConfig struct {
	specifications string
	logography     string
	dictionary     string
}

type config struct {
	name              string
	debugEnabled      bool
	broadcastTestData bool
	files             filePathConfig
	procedures        procedureConfig
}

type dictExtractMethod int

const (
	extractWithRegex dictExtractMethod = 0
	extractWithAgent dictExtractMethod = 1
)

func (d dictExtractMethod) String() string {
	return [...]string{
		"EXTRACT_WITH_REGEX",
		"EXTRACT_WITH_AGENT",
	}[d]
}
