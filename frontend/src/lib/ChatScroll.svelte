<script lang="ts">
	import { tick } from 'svelte';
	import Markdown from './Markdown.svelte';

	interface Props {
		src: string;
	}
	const { src }: Props = $props();

	let messages: Message[] = $state.raw([]);

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
				if (json.sender !== 'SERVER') {
					messages = [...messages, json];
				}
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

<ul class="h-1/2 overflow-x-clip overflow-y-auto" bind:this={element}>
	{#each messages as { timestamp, text, sender, command }, i (i)}
		{#if command == 0}
			<li class="m-2 border-b-1 p-2 font-mono text-sm">
				<strong>{timestamp} {sender}</strong>:
				<article>
					{#key text}
						<Markdown source={text} />
					{/key}
				</article>
			</li>
		{/if}
	{/each}
</ul>
