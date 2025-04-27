<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		children?: Snippet;
	}
	const { children }: Props = $props();
</script>

<ol class="list-decimal">{@render children?.()}</ol>
