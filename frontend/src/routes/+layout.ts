import type { LayoutLoad } from './$types';

export const load: LayoutLoad = () => {
	const host = 'http://localhost:7070';
	const generationsSrc = host + '/generations';
	const logographyDisplay = host + '/logograms/display';
	const dictionarySrc = host + '/dictionary';
	const chatSrc = host + '/chat';
	return {
		host,
		generationsSrc,
		dictionarySrc,
		logographyDisplay,
		chatSrc
	};
};
