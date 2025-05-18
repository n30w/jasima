<script lang="ts">
	import { fade, fly } from 'svelte/transition';
	import type { PageProps } from './$types';
	import { quintOut } from 'svelte/easing';
	import { typewriter } from '$lib/typewriter';

	const { data }: PageProps = $props();

	const src = data.logographyDisplay;

	let advRes: string = $state('');
	let genRes: string = $state('');
	let svg: string = $state.raw('');

	$effect(() => {
		const es = new EventSource(src);
		es.onmessage = (event) => {
			try {
				const json: LogogramIteration = JSON.parse(event.data);
				svg = json.generator.svg;
				advRes = json.adversary.response;
				genRes = json.generator.response;
			} catch (err) {
				console.log('Failed to parse logogram', err);
			}
		};

		es.onerror = (err) => {
			console.error('SSE error:', err);
			// es.close(); // Optional: close on error
		};

		return () => es.close(); // Clean up when component unmounts
	});
</script>

<div class="">
	<div class="flex min-h-screen flex-col items-center justify-center">
		<div class="border-1 h-1/2 w-1/2 border-dashed">
			{#key svg}
				<div transition:fly={{ duration: 300, x: -300, y: -500, easing: quintOut }}>
					{@html svg}
				</div>
			{/key}
		</div>
	</div>
	<div class="absolute left-10 top-10 w-1/4">
		<h1 class="pb-2 font-light tracking-widest">GENERATOR</h1>
		{#key genRes}
			<p class="text-sm" in:typewriter={{ speed: 20 }}>{genRes}</p>
		{/key}
	</div>
	<div class="absolute bottom-10 right-10 w-1/4">
		<h1 class="pb-2 text-right font-light uppercase tracking-widest">Adversary</h1>
		{#key advRes}
			<p class="text-right text-sm" in:typewriter={{ speed: 20 }}>{advRes}</p>
		{/key}
	</div>
</div>
