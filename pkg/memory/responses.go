package memory

type ResponseStop struct {
	Stop bool `json:"stop" jsonschema_description:"Indicates if you want to end the conversation"`
}

type ResponseText struct {
	Response string `json:"response" jsonschema_description:"Your response"`
}

type ResponseLogogramIteration struct {
	Name         string `json:"name" jsonschema_description:"Logogram name"`
	Svg          string `json:"svg" jsonschema_description:"Logogram svg"`
	ResponseText `jsonschema_description:"Reasoning and explanation of iteration"`
	ResponseStop
}

type ResponseLogogramCritique struct {
	Name         string `json:"name" jsonschema_description:"Logogram name"`
	ResponseText `jsonschema_description:"Critique of logogram"`
	ResponseStop
}

type ResponseDictionaryWordsDetection struct {
	Words []string `json:"words" jsonschema_description:"Words in the dictionary from the text"`
}

type LogogramIteration struct {
	Generator ResponseLogogramIteration `json:"generator"`
	Adversary ResponseLogogramCritique  `json:"adversary"`
}

type ResponseDictionaryEntryUpdate struct {
	dictionaryEntry

	// Remove represents whether a word should be removed from the dictionary.
	// This is used when sending data to and from an agent. If an agent is
	// queried to remove an entry from the dictionary, this field would be
	// set to `true`.
	Remove bool `json:"remove" jsonschema_description:"Remove word"`
}

type ResponseDictionaryEntries struct {
	Entries []ResponseDictionaryEntryUpdate `json:"entries" jsonschema_description:"Dictionary entries"`
}
