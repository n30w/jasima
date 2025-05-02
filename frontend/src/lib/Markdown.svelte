<script lang="ts">
	import SvelteMarkdown from '@humanspeak/svelte-markdown';
	import CustomUnorderedList from '$lib/CustomUnorderedList.svelte';
	import CustomListItem from '$lib/CustomListItem.svelte';
	import CustomOrderedList from '$lib/CustomOrderedList.svelte';
	import CustomParagraph from '$lib/CustomParagraph.svelte';

	interface Props {
		source?: string;
	}

	const { source }: Props = $props();
</script>

<SvelteMarkdown
	source={source ? source : ``}
	renderers={{
		list: CustomUnorderedList,
		listitem: CustomListItem,
		paragraph: CustomParagraph
	}}
/>
