<script lang="ts">
	import { draw, fade } from 'svelte/transition';
	import type { PageProps } from './$types';
	import { quintInOut, quintOut } from 'svelte/easing';
	const { data }: PageProps = $props();
	const src = data.generationsSrc;

	let generations: Generation[] = $state([]);
	let logos: Map<string, string> = $state(new Map<string, string>());

	import { animate, svg, stagger } from 'animejs';

	$effect(() => {
		animate(svg.createDrawable('.line'), {
			draw: ['0 0', '0 1', '1 1'],
			ease: 'inOutQuad',
			duration: 2000,
			delay: stagger(100),
			loop: true
		});
	});

	$effect(() => {
		const es = new EventSource(src);
		es.onmessage = (event) => {
			try {
				const json: Generation = JSON.parse(event.data);
				generations = [...generations, json];
				for (const [key, value] of Object.entries(json.logography)) {
					logos.set(key, value);
				}
				logos = new Map(logos);
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

<div class="mx-auto grid grid-cols-10 gap-2">
	{#each logos as [key, value] (key)}
		{#key key}
			<div
				class="flex flex-col-reverse items-center border-1 p-3"
				transition:fade={{ delay: 100, duration: 500, easing: quintOut }}
			>
				<h1>{key}</h1>
				<div class="h-20 w-20">
					{@html value}
				</div>
			</div>
		{/key}
	{/each}
</div>
