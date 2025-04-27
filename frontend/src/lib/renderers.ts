import type { HtmlRenderers, Renderers } from '@humanspeak/svelte-markdown';
import CustomParagraph from './CustomParagraph.svelte';
import Blockquote from '@humanspeak/svelte-markdown';
import Code from '@humanspeak/svelte-markdown';
import CustomOrderedList from './CustomOrderedList.svelte';
import CustomUnorderedList from './CustomUnorderedList.svelte';
import CustomListItem from './CustomListItem.svelte';

export const customHtmlRenderers: Partial<Renderers> = {
	html: {
		ol: CustomOrderedList,
		ul: CustomUnorderedList,
		li: CustomListItem
	},
	blockquote: Blockquote,
	code: Code,
	paragraph: CustomParagraph
};
