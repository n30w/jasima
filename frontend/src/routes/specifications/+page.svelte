<script lang="ts">
	import Markdown from '$lib/Markdown.svelte';
	import { fly } from 'svelte/transition';
	import type { PageProps } from './$types';
	const { data }: PageProps = $props();
	const src = data.generationsSrc;

	let generations: Generation[] = $state.raw([]);
	let currentPhoneticsSpec: string = $state.raw('');
	let currentGrammarSpec: string = $state.raw('');
	let currentLogographySpec: string = $state.raw('');
	let currentDictionarySpec: string = $state.raw('');

	$effect(() => {
		const es = new EventSource(src);
		es.onmessage = (event) => {
			try {
				const json: Generation = JSON.parse(event.data);
				generations = [...generations, json];
				currentPhoneticsSpec = json.specifications['1'];
				currentDictionarySpec = json.specifications['3'];
				currentGrammarSpec = json.specifications['2'];
				currentLogographySpec = json.specifications['4'];
			} catch (err) {
				console.log('Failed to parse JSON from /generations', err);
			}
		};

		es.onerror = (err) => {
			console.error('SSE error:', err);
			// es.close(); // Optional: close on error
		};

		return () => es.close(); // Clean up when component unmounts
	});
</script>

<div class="flex grow-0">
	<section>
		{#key currentPhoneticsSpec}
			<div in:fly={{ duration: 500, y: -200 }}>
				<Markdown source={currentPhoneticsSpec} />
			</div>
		{/key}
	</section>
	<section>
		{#key currentGrammarSpec}
			<div in:fly={{ duration: 500, y: 200 }}>
				<Markdown source={currentGrammarSpec} />
			</div>
		{/key}
	</section>
	<section>
		{#key currentDictionarySpec}
			<div in:fly={{ duration: 500, y: -200 }}>
				<Markdown source={currentDictionarySpec} />
			</div>
		{/key}
	</section>
	<section>
		{#key currentLogographySpec}
			<div in:fly={{ duration: 500, y: 200 }}>
				<Markdown source={currentLogographySpec} />
			</div>
		{/key}
	</section>
</div>
