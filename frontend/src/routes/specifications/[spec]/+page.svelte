<script lang="ts">
	import { fly } from 'svelte/transition';
	import type { PageProps } from './$types';
	import Markdown from '$lib/Markdown.svelte';
	let { data }: PageProps = $props();
	const src = data.host + '/specifications';

	let specs: Map<string, string> = $state.raw(new Map<string, string>());
	let spec: string = $state.raw('');

	$effect(() => {
		const es = new EventSource(src);
		es.onmessage = (event) => {
			try {
				const json: Specifications = JSON.parse(event.data);
				for (const [key, value] of Object.entries(json)) {
					specs.set(key, value);
				}
				specs = new Map<string, string>(specs);
				spec = specs.get(data.slug)!;
				console.log(json);
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
</script>

<div class="flex min-h-screen flex-col items-center justify-center">
	<div class="mx-auto w-3/4">
		{#key spec}
			<div in:fly={{ duration: 500, y: 200 }}>
				<Markdown source={spec} />
			</div>
		{/key}
	</div>
</div>
