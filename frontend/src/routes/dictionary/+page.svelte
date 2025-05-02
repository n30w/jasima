<script lang="ts">
	const src = 'http://127.0.0.1:7070/test/generations';

	let currentDict: dictionaryEntry[] = $state([]);

	$effect(() => {
		const es = new EventSource(src);
		es.onmessage = (event) => {
			try {
				const json: generation = JSON.parse(event.data);
				const nd = [];
				for (const [key, value] of Object.entries(json.dictionary)) {
					nd.push(value);
				}
				currentDict = nd;
				console.log(json);
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

<div class="grid grid-cols-16 gap-1">
	{#each currentDict as d, i (i)}
		<div class="border-1 p-2 hover:bg-neutral-100">
			<h1 class="text-lg font-bold">{d.word}</h1>
			<p class="text-xs">{d.definition}</p>
		</div>
	{/each}
</div>
