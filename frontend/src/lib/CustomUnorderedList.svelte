<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		children?: Snippet;
	}
	const { children }: Props = $props();
</script>

<ul class="mx-2 list-disc px-1">{@render children?.()}</ul>
