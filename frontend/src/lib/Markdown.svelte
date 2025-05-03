<script lang="ts">
	import SvelteMarkdown, { type Token } from '@humanspeak/svelte-markdown';
	import CustomUnorderedList from '$lib/CustomUnorderedList.svelte';
	import CustomListItem from '$lib/CustomListItem.svelte';
	import CustomOrderedList from '$lib/CustomOrderedList.svelte';
	import CustomParagraph from '$lib/CustomParagraph.svelte';

	interface Props {
		source?: string;
	}

	const { source }: Props = $props();

	const handleParsed = (tokens: Token[] | DOMTokenList) => {
		// console.log('Parsed tokens:', tokens);
	};
</script>

{#key source}
	<SvelteMarkdown
		source={source ? source : ``}
		renderers={{
			list: CustomUnorderedList,
			listitem: CustomListItem,
			paragraph: CustomParagraph
		}}
		parsed={handleParsed}
	/>
{/key}
