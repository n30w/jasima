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
};

type Dictionary = Map<string, DictionaryEntry>;

type Specifications = Map<string, string>;

type Generation = {
	transcript: Map<string, string>;
	logography: Map<string, string>;
	specifications: Map<number, string>;
	dictionary: Dictionary;
};
