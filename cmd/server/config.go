package main

type procedureConfig struct {
	// maxExchanges represents the total exchanges allowed per layer
	// of evolution.
	maxExchanges int

	// maxGenerations represents the maximum number of generations to evolve.
	// When set to 0, the procedure evolves forever.
	maxGenerations int
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
