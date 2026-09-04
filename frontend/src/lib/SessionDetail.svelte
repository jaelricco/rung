<script>
	// One session, written out in the order it is to be performed. The same
	// component serves the plan preview and the calendar, so a session looks
	// the same whether it is being considered or being done.
	let { session } = $props();

	const INTENTS = {
		prep: 'Prep',
		skill: 'Skill',
		strength: 'Strength',
		accessory: 'Accessory',
		conditioning: 'Conditioning'
	};

	let blocks = $derived(session?.blocks ?? []);
</script>

{#if session?.warmup_protocols?.length}
	<p class="muted warmup">Warm-up: {session.warmup_protocols.join(', ')}</p>
{/if}

<table>
	<tbody>
		{#each blocks as block, i (i)}
			<tr>
				<td class="slug">
					{block.exercise_slug}
					{#if block.intent}
						<span class="chip">{INTENTS[block.intent] ?? block.intent}</span>
					{/if}
				</td>
				<td class="mono">{block.sets} × {block.prescription}</td>
				<td class="mono muted rest">
					{block.rest_seconds ? `${block.rest_seconds}s rest` : ''}
				</td>
			</tr>
			{#if block.intensity || block.tempo || block.progression || block.notes}
				<tr>
					<td colspan="3" class="detail muted">
						{#if block.intensity}<span class="mono">{block.intensity}</span>{/if}
						{#if block.tempo}<span class="mono">tempo {block.tempo}</span>{/if}
						{#if block.notes}<span>{block.notes}</span>{/if}
						{#if block.progression}<span class="next">Next week: {block.progression}</span>{/if}
					</td>
				</tr>
			{/if}
		{/each}
	</tbody>
</table>

{#if session?.cooldown}
	<p class="muted cooldown">Cool-down: {session.cooldown}</p>
{/if}

<style>
	.warmup {
		font-size: 0.85rem;
		margin: 0 0 0.5rem;
	}
	.slug {
		width: 40%;
	}
	.rest {
		width: 20%;
	}
	.chip {
		margin-left: 0.35rem;
	}
	/* The second row of a block: everything that says how hard, how fast and
	   what changes next week. Separated so the prescription itself stays
	   scannable. */
	.detail {
		font-size: 0.85rem;
		padding-top: 0;
		display: flex;
		flex-wrap: wrap;
		gap: 0.15rem 0.9rem;
	}
	.detail .next {
		color: var(--signal-text);
		opacity: 0.85;
	}
	.cooldown {
		font-size: 0.85rem;
		margin: 0.6rem 0 0;
	}
</style>
