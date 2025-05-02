<script lang="ts">
	import '../app.css';

	let { children } = $props();
</script>

<main class="h-screen">
	{@render children()}
</main>
