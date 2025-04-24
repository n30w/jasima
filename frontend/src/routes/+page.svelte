<script lang="ts">
	import { EventSource } from 'eventsource';
	import { tick } from 'svelte';
	import SvelteMarkdown from '@humanspeak/svelte-markdown';

	const src = 'http://127.0.0.1:7070/events';

	let messages: { message; timestamp: string }[] = $state.raw([]);
	let element: HTMLElement;

	$effect.pre(() => {
		const autoscroll = messages && element;
		if (autoscroll) {
			tick().then(() => {
				element.scroll({ top: element.scrollHeight, behavior: 'smooth' });
			});
		}
	});

	$effect(() => {
		const es = new EventSource(src);

		es.onmessage = (event) => {
			try {
				const json = JSON.parse(event.data);
				messages = [...messages, json];
			} catch (err) {
				console.error('Failed to parse JSON from event:', err);
			}
		};

		es.onerror = (err) => {
			console.error('SSE error:', err);
			// es.close(); // Optional: close on error
		};

		return () => es.close(); // Clean up when component unmounts
	});
</script>

<!-- <div class="h-1/2"> -->
<ul class="h-screen overflow-auto" bind:this={element}>
	{#each messages as { timestamp, text, sender, command }, i (i)}
		{#if command == 0}
			<li class="m-2 border-b-1 p-2 font-mono text-base">
				<strong>{timestamp} {sender}</strong>:
				<article>
					<SvelteMarkdown source={text} />
				</article>
			</li>
		{/if}
	{/each}
</ul>
<!-- </div> -->
