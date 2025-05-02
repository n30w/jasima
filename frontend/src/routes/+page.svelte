<script lang="ts">
	/** eslint-disable svelte/no-at-html-tags */
	import { EventSource } from 'eventsource';
	import { tick } from 'svelte';
	import { TypeWriter } from 'svelte-typewrite';
	import Typewriter from 'svelte-typewriter';
	import { animate, stagger, utils } from 'animejs';
	import Markdown from '$lib/Markdown.svelte';

	// const src = 'http://127.0.0.1:7070/chat';
	const src2 = 'http://127.0.0.1:7070/test/generations';
	const src = 'http://127.0.0.1:7070/test/chat';

	let currentDict: { word: string; definition: string; remove: boolean }[] = $state([]);
	let currentSpecs: specifications = $state.raw({});
	let currentPhoneticsSpec: string = $state.raw('');
	let currentGrammarSpec: string = $state.raw('');
	let currentLogographySpec: string = $state.raw('');
	let currentDictionarySpec: string = $state.raw('');
	let logography: Map<string, string> = $state.raw({});
	let logos: string[] = $state([]);

	let generations: generation[] = $state.raw([]);
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
		const es = new EventSource(src2);
		es.onmessage = (event) => {
			try {
				const json: generation = JSON.parse(event.data);
				generations = [...generations, json];
				currentSpecs = json.specifications;
				currentPhoneticsSpec = json.specifications['1'];
				currentDictionarySpec = json.specifications['3'];
				currentGrammarSpec = json.specifications['2'];
				currentLogographySpec = json.specifications['4'];
				logography = new Map(Object.entries(json.logography));
				for (const [key, value] of Object.entries(json.logography)) {
					if (!logos.includes(value)) {
						logos.push(value);
					}
				}
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

<!-- <Typewriter cursor={false} mode="scramble" scrambleDuration={500}> -->

<!-- <SvelteMarkdown source={text} /> -->
<!-- </Typewriter> -->

<ul class="h-1/4 overflow-x-clip overflow-y-auto" bind:this={element}>
	{#each messages as { timestamp, text, sender, command }, i (i)}
		{#if command == 0}
			<li class="m-2 border-b-1 p-2 font-mono text-sm">
				<strong>{timestamp} {sender}</strong>:
				<article>
					<p>
						<Markdown source={text} />
					</p>
				</article>
			</li>
		{/if}
	{/each}
</ul>

<!-- <div class="mx-auto grid w-1/2 grid-cols-10">
		{#each logos as logo, i (i)}
			<div class="h-15 w-15">{@html logo}</div>
		{/each}
	</div> -->
<div class="grid grid-cols-4">
	<div>
		<TypeWriter texts={[currentPhoneticsSpec]} typeSpeed={10} />
	</div>
	<div>
		<TypeWriter texts={[currentGrammarSpec]} typeSpeed={10} />
		<!-- <Markdown source={currentGrammarSpec} /> -->
	</div>
	<div>
		<TypeWriter texts={[currentDictionarySpec]} typeSpeed={10} />
		<!-- <Markdown source={currentDictionarySpec} /> -->
	</div>
	<div>
		<TypeWriter texts={[currentLogographySpec]} typeSpeed={10} />
		<!-- <Markdown source={currentLogographySpec} /> -->
	</div>
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
