import type { LayoutLoad } from './$types';

export const load: LayoutLoad = () => {
	return {
		generationsSrc: 'http://localhost:7070/test/generations'
	};
};
