<script lang="ts">
	import { typewriter } from '$lib/typewriter';
	import { fade } from 'svelte/transition';
	import type { PageProps } from './$types';
	const { data }: PageProps = $props();

	const src = data.generationsSrc;

	let currDict: Map<string, DictionaryEntry> = $state(new Map<string, DictionaryEntry>());
	let currLog: Map<string, string> = $state(new Map<string, string>());

	$effect(() => {
		const es = new EventSource(src);
		es.onmessage = (event) => {
			try {
				const json: Generation = JSON.parse(event.data);
				for (const [key, value] of Object.entries(json.dictionary)) {
					currDict.set(key, value);
				}
				for (const [key, value] of Object.entries(json.logography)) {
					currLog.set(key, value);
				}
				currDict = new Map(currDict);
				currLog = new Map(currLog);
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

<div class="flex flex-wrap gap-1">
	{#each currDict as [key, value] (key)}
		<div class="h-fit max-w-1/3 border-1 p-2 hover:bg-neutral-100">
			<div class="h-10 w-10">
				{#key key}
					{@html currLog.get(value.word)}
				{/key}
			</div>
			<h1 class="text-base font-bold">
				{#key value.word}
					<span in:fade>{value.word}</span>
				{/key}
			</h1>
			{#key value.definition}
				<p class="text-xs" in:typewriter={{ speed: 10 }}>
					{value.definition}
				</p>
			{/key}
		</div>
	{/each}
</div>
