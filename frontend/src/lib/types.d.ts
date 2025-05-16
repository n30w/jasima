type Message = {
	text?: string;
	timestamp: string;
	sender: string;
	command: number;
};

type DictionaryEntry = {
	word: string;
	definition: string;
	remove: boolean;
	logogram: string;
};

type Dictionary = Map<string, DictionaryEntry>;

type Specifications = Map<string, string>;

type Generation = {
	transcript: Map<string, string>;
	logography: Map<string, string>;
	specifications: Map<number, string>;
	dictionary: Dictionary;
};

type UsedWords = {
	words: string[];
};

type agentLogogramIterResponse = {
	name: string;
	response: string;
	stop: boolean;
};

type agentLogogramAdversaryResp = agentLogogramIterResponse;

type agentLogogramGeneratorResp = {
	svg: string;
	agentLogogramIterResponse;
};

type LogogramIteration = {
	generator: agentLogogramGeneratorResp;
	adversary: agentLogogramAdversaryResp;
};
