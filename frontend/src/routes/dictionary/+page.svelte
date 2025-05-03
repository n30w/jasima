<script lang="ts">
	import { typewriter } from '$lib/typewriter';
	import { fade } from 'svelte/transition';

	const src = 'http://127.0.0.1:7070/test/generations';

	let currDict: Map<string, DictionaryEntry> = $state(new Map<string, DictionaryEntry>());

	$effect(() => {
		const es = new EventSource(src);
		es.onmessage = (event) => {
			try {
				const json: Generation = JSON.parse(event.data);
				for (const [key, value] of Object.entries(json.dictionary)) {
					currDict.set(key, value);
				}
				currDict = new Map(currDict);
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
		{#if key}
			<div class="h-fit max-w-1/3 border-1 p-2 hover:bg-neutral-100">
				<h1 class="text-base font-bold">
					{#key value.word}
						<!-- fade the word whenever it changes -->
						<span in:fade>{value.word}</span>
					{/key}
				</h1>
				{#key value.definition}
					<!-- fade the definition whenever it changes -->
					<p class="text-xs" in:typewriter={{ speed: 10 }}>
						{value.definition}
					</p>
				{/key}
			</div>
		{/if}
	{/each}
</div>
