type message = {
	text?: string;
	timestamp: string;
	sender: string;
	command: number;
};

type dictionaryEntry = {
	word: string;
	definition: string;
	remove: boolean;
};

type dictionary = Map<string, dictionaryEntry>;

type specifications = Map<number, string>;

type generation = {
	transcript: Map<string, string>;
	logography: Map<string, string>;
	specifications: Map<number, string>;
	dictionary: dictionary;
};
