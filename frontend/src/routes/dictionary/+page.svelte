<script lang="ts">
	import { typewriter } from '$lib/typewriter';
	import { fade } from 'svelte/transition';
	import type { PageProps } from './$types';
	const { data }: PageProps = $props();

	const src = data.generationsSrc;

	const src2 = data.host + '/wordDetection';

	let currUsed: string[] = $state([]);
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
			es.close(); // Optional: close on error
		};

		return () => es.close(); // Clean up when component unmounts
	});

	$effect(() => {
		const es2 = new EventSource(src2);
		es2.onmessage = (event) => {
			try {
				const json: UsedWords = JSON.parse(event.data);
				currUsed = [...json.words];
				// console.log(json);
			} catch (err) {
				console.log(err);
			}
		};

		es2.onerror = (err) => {
			console.error('SSE error:', err);
			es2.close();
		};

		return () => es2.close();
	});

	const isCurrUsed = (v: string): boolean => currUsed.includes(v);
	const getStagger = (): string => {
		let offset = Math.random();
		if (offset < 0.4) {
			offset = 0.4;
		}

		return ``;
	};
</script>

<div class="flex flex-wrap gap-1 p-2">
	{#each currDict as [key, value] (key)}
		<div
			class={[
				'max-w-1/3 border-1 h-fit p-2 transition-all duration-150 ease-in',
				isCurrUsed(value.word) && 'bg-green-300'
			]}
		>
			{#if currLog.has(value.word)}
				<div class="h-10 w-10">
					{#key key}
						{@html currLog.get(value.word)}
					{/key}
				</div>
			{/if}
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
