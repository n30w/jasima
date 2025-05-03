<script lang="ts">
	import type { Snippet } from 'svelte';
	import { typewriter } from './typewriter';

	interface Props {
		children?: Snippet;
	}

	const { children }: Props = $props();
</script>

<p in:typewriter={{ speed: 10 }} class="m-1 p-1">
	{@render children?.()}
</p>
