<script>
	// The loading bar for anything the model is doing. Percent comes from the
	// backend's progress events — reasoning that has arrived, sessions that
	// have been written — so it tracks real work rather than a timer. Pass
	// percent as null for the one call that cannot report a fraction (web
	// search), and the bar sweeps instead of filling rather than inventing one.
	let { label = 'Working', percent = 0, detail = '', done = 0, total = 0 } = $props();

	let determinate = $derived(typeof percent === 'number');
	let width = $derived(determinate ? Math.min(100, Math.max(0, Math.round(percent))) : 0);
</script>

<div
	class="ai-progress"
	role="progressbar"
	aria-label={label}
	aria-valuemin={determinate ? 0 : undefined}
	aria-valuemax={determinate ? 100 : undefined}
	aria-valuenow={determinate ? width : undefined}
	aria-valuetext={determinate ? `${width}% — ${label}` : label}
>
	<div class="head">
		<span class="eyebrow spark">AI at work</span>
		<span class="label">{label}</span>
		<span class="pct mono">
			{#if determinate}{width}%{:else}—{/if}
		</span>
	</div>

	<div class="track" class:sweeping={!determinate}>
		<div class="fill" style:width="{width}%"></div>
	</div>

	<p class="meta mono muted">
		{#if done > 0 && total > 0}
			{done} / {total} sessions written
		{:else if detail}
			{detail}
		{:else}
			&nbsp;
		{/if}
	</p>
</div>

<style>
	.ai-progress {
		border: 1px solid var(--line);
		border-radius: var(--radius);
		background: var(--panel);
		padding: 0.85rem 1rem 0.7rem;
	}

	.head {
		display: flex;
		align-items: baseline;
		gap: 0.7rem;
	}

	/* A blinking marker, so it is obvious at a glance that something is
	   running and not merely drawn. */
	.spark::before {
		content: '';
		display: inline-block;
		width: 6px;
		height: 6px;
		margin-right: 0.45rem;
		border-radius: 50%;
		background: var(--signal);
		animation: pulse 1.4s ease-in-out infinite;
	}

	.label {
		flex: 1;
		font-size: 0.92rem;
	}

	.pct {
		font-size: 1.05rem;
		font-weight: 600;
		color: var(--signal);
		font-variant-numeric: tabular-nums;
	}

	.track {
		position: relative;
		height: 8px;
		margin-top: 0.6rem;
		background: var(--ink);
		border: 1px solid var(--line);
		border-radius: 1px;
		overflow: hidden;
	}

	.fill {
		height: 100%;
		background: var(--signal);
		transition: width 0.4s ease;
	}

	/* No percentage to show: sweep the width of the track instead. */
	.track.sweeping::after {
		content: '';
		position: absolute;
		inset: 0 auto 0 0;
		width: 35%;
		background: var(--signal);
		animation: sweep 1.6s ease-in-out infinite;
	}

	.meta {
		font-size: 0.76rem;
		margin: 0.45rem 0 0;
		min-height: 1.1em;
		overflow-wrap: anywhere;
	}

	@keyframes pulse {
		0%,
		100% {
			opacity: 1;
		}
		50% {
			opacity: 0.25;
		}
	}

	@keyframes sweep {
		0% {
			transform: translateX(-100%);
		}
		100% {
			transform: translateX(285%);
		}
	}

	/* app.css disables animation under prefers-reduced-motion; the bar still
	   fills, it just jumps rather than slides. */
	@media (prefers-reduced-motion: reduce) {
		.track.sweeping::after {
			width: 100%;
			opacity: 0.35;
		}
	}
</style>
