<script lang="ts">
	import { EventSource } from 'eventsource';
	import { tick } from 'svelte';
	import SvelteMarkdown from '@humanspeak/svelte-markdown';
	// import Typewriter from 'svelte-typewriter';
	import { TypeWriter } from 'svelte-typewrite';
	import { animate, stagger, utils } from 'animejs';
	import { customHtmlRenderers } from '$lib';
	import CustomUnorderedList from '$lib/CustomUnorderedList.svelte';
	import CustomListItem from '$lib/CustomListItem.svelte';
	import CustomOrderedList from '$lib/CustomOrderedList.svelte';
	import CustomParagraph from '$lib/CustomParagraph.svelte';
	import Heading from '@humanspeak/svelte-markdown';

	const src = 'http://127.0.0.1:7070/chat';
	// const src = 'http://127.0.0.1:7070/test/chat';

	type message = {
		text?: string;
		timestamp: string;
		sender: string;
		command: number;
	};

	let messages: message[] = $state.raw([]);
	let val: message = $state.raw({
		text: '',
		timestamp: '',
		sender: '',
		command: 0
	});
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
					val = json;
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

	// function animateGrid() {
	// 	animate('.square', {
	// 		scale: [{ to: [0, 1.25] }, { to: 0 }],
	// 		boxShadow: [{ to: '0 0 1rem 0 currentColor' }, { to: '0 0 0rem 0 currentColor' }],
	// 		delay: stagger(100, {
	// 			grid: [11, 4],
	// 			from: utils.random(0, 11 * 4)
	// 		}),
	// 		onComplete: animateGrid
	// 	});
	// }

	// $effect(() => {
	// 	animate('.square', {
	// 		x: window.innerWidth - 100,
	// 		rotate: { from: -180 },
	// 		duration: 4000,
	// 		delay: stagger(135, { from: 'first' }),
	// 		ease: 'inOutQuint',
	// 		loop: true,
	// 		alternate: true
	// 	});
	// 	// animateGrid();
	// });
</script>

<div class="h-screen">
	<!-- <Typewriter cursor={false} mode="scramble" scrambleDuration={500}> -->

	<!-- <SvelteMarkdown source={text} /> -->
	<!-- </Typewriter> -->
	<ul class="h-screen overflow-x-clip overflow-y-auto" bind:this={element}>
		{#each messages as { timestamp, text, sender, command }, i (i)}
			{#if command == 0}
				<li class="m-2 border-b-1 p-2 font-mono text-sm">
					<strong>{timestamp} {sender}</strong>:
					<article>
						<p>
							<SvelteMarkdown
								source={text ? text : ``}
								renderers={{
									heading: Heading,
									list: CustomUnorderedList,
									listitem: CustomListItem,
									paragraph: CustomParagraph
								}}
							/>
						</p>
					</article>
				</li>
			{/if}
		{/each}
	</ul>
</div>

<!-- <div class="absolute top-0">
	<div class="square m-4 h-20 w-20 bg-orange-200"></div>
	<div class="square m-4 h-20 w-20 bg-orange-300"></div>
	<div class="square m-4 h-20 w-20 bg-orange-100"></div>
	<div class="square m-4 h-20 w-20 bg-orange-300"></div>
	<div class="square m-4 h-20 w-20 bg-orange-400"></div>
	<div class="square m-4 h-20 w-20 bg-orange-300"></div>
	<div class="square m-4 h-20 w-20 bg-orange-200"></div>
	<div class="square m-4 h-20 w-20 bg-orange-100"></div>
</div> -->
