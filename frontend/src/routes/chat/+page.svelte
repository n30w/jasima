<script lang="ts">
	/** eslint-disable svelte/no-at-html-tags */
	import { EventSource } from 'eventsource';
	import { typewriter } from '$lib/typewriter';
	import type { PageProps } from './$types';

	const { data }: PageProps = $props();

	// const src = 'http://127.0.0.1:7070/chat';
	// const src = data.chatSrc;
	const src = '/api/chat-proxy';

	let val: Message = $state.raw({
		text: '',
		timestamp: '',
		sender: '',
		command: 0
	});
	$effect(() => {
		const es = new EventSource(src);

		es.onmessage = (event) => {
			try {
				const json = JSON.parse(event.data);
				if (json.sender !== 'SERVER') {
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

<!-- <MorphBall /> -->
<div class="mx-auto w-1/2">
	{#key val}
		<!-- <Markdown source={val.text} /> -->
		<h1 in:typewriter={{ speed: 10 }} class="font-bold">{val.sender}</h1>
		<p in:typewriter={{ speed: 100 }}>{val.text}</p>
	{/key}
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
