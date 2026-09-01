<script>
	// One error box for every page that talks to a model. Most failures are
	// just failures; the one worth acting on is "you have not connected an AI
	// account yet", which the API answers with 428 and which is a link, not an
	// apology.
	let { error = null, style = 'margin-top:1rem' } = $props();

	let needsKey = $derived(error?.status === 428);
</script>

{#if error}
	<div class="notice error" {style}>
		{error.message}
		{#if needsKey}
			<p style="margin:0.55rem 0 0">
				<a href="/settings">Connect your Claude or ChatGPT account →</a>
			</p>
		{/if}
	</div>
{/if}
