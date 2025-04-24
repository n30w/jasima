<script lang="ts">
	import { EventSource } from 'eventsource';
	import { tick } from 'svelte';

	// const eventSrc = new EventSource('http://127.0.0.1:7070/time');
	const src = 'http://127.0.0.1:7070/events';
	const src2 = 'http://127.0.0.1:7070/time';

	let messages: { message; timestamp: string }[] = $state.raw([]);
	let element: HTMLElement;

	$effect.pre(() => {
		const autoscroll =
			messages && element && element.offsetHeight + element.scrollTop > element.scrollHeight - 50;
		if (autoscroll) {
			tick().then(() => {
				element.scroll({ top: element.scrollHeight, behavior: 'smooth' });
			});
		}
	});

	$effect(() => {
		const es = new EventSource(src);

		es.onmessage = (event) => {
			// console.log(messages);
			try {
				const json = JSON.parse(event.data);
				// console.log(json);
				messages = [...messages, json];
			} catch (err) {
				console.error('Failed to parse JSON from event:', err);
			}
			// messages = [...messages, event.data];
			// v = event.data;
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
	{#each messages as { message, timestamp, text, sender, command }, i (i)}
		<!-- {#if command == 0} -->
		<li class="m-2 border-b-1 p-2 text-lg"><strong>{timestamp} {sender}</strong>: {text}</li>
		<!-- {/if} -->
	{/each}
</ul>
<!-- </div> -->
