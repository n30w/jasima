package main

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
