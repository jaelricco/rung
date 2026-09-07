<script>
	// One session, laid out in the order it is performed. This is the page you
	// open in a second tab and read off in the gym, which is what decides
	// nearly everything about it: the warm-up is written out as the steps you
	// actually do rather than named as a protocol slug, movements carry their
	// real names, and the six phases are always all six — a phase with nothing
	// in it says so, because a missing heading reads as "skip this" and the
	// phase that gets skipped is the one nobody sees.
	import { api } from '$lib/api.js';
	import { page } from '$app/state';
	import { formatDate } from '$lib/week.js';
	import Failure from '$lib/Failure.svelte';

	const INTENTS = {
		prep: 'Prep',
		skill: 'Skill',
		strength: 'Strength',
		accessory: 'Accessory',
		conditioning: 'Conditioning'
	};

	let view = $state(null);
	let error = $state(null);
	let busy = $state(false);

	$effect(() => {
		const id = page.params.id;
		let cancelled = false;
		(async () => {
			try {
				const loaded = await api.get(`/sessions/${id}`);
				if (!cancelled) {
					view = loaded;
					error = null;
				}
			} catch (e) {
				if (!cancelled) error = e;
			}
		})();
		return () => (cancelled = true);
	});

	let entry = $derived(view?.session ?? null);
	let body = $derived(entry?.body ?? null);
	let blocks = $derived(body?.blocks ?? []);
	let protocols = $derived(view?.protocols ?? []);

	// A rehab protocol is not a phase of a warm-up — it is the reason parts of
	// the session are what they are, so it sits above the whole thing rather
	// than inside one of the six.
	let rehab = $derived(protocols.filter((p) => p.purpose === 'rehab'));

	// Prep blocks are the specific warm-up performed as sets: ramp-ups and
	// joint work for the movement this session is built on. Everything else is
	// the training.
	let prepBlocks = $derived(blocks.filter((b) => b.intent === 'prep'));
	let workBlocks = $derived(blocks.filter((b) => b.intent !== 'prep'));

	function protocolsFor(key) {
		return protocols.filter((p) => p.purpose !== 'rehab' && p.phase === key);
	}

	let phases = $derived(
		(view?.phases ?? []).map((phase) => {
			const items = protocolsFor(phase.key);
			const sets = phase.key === 'specific' ? prepBlocks : phase.key === 'training' ? workBlocks : [];
			const text = phase.key === 'cooldown' ? (body?.cooldown ?? '') : '';
			return { ...phase, items, sets, text, empty: !items.length && !sets.length && !text };
		})
	);

	function nameOf(slug) {
		return view?.exercises?.[slug]?.name ?? slug;
	}

	async function toggleDone() {
		if (!entry) return;
		busy = true;
		try {
			// The endpoint answers with a status word rather than the updated
			// row, so the new state is written here rather than read back.
			const path = `/sessions/${entry.id}/complete`;
			let completed_at = null;
			if (entry.completed_at) {
				await api.del(path);
			} else {
				await api.post(path, {});
				completed_at = new Date().toISOString();
			}
			view = { ...view, session: { ...entry, completed_at } };
			error = null;
		} catch (e) {
			error = e;
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>{entry?.title ? `${entry.title} — rung` : 'Session — rung'}</title>
</svelte:head>

{#if error}
	<Failure {error} />
{/if}

{#if entry}
	<header class="head">
		<p class="eyebrow" style="margin:0">{formatDate(entry.scheduled_on)}</p>
		<h1 style="margin:0.2rem 0 0">{entry.title}</h1>
		{#if entry.focus}
			<p class="lede" style="margin:0.35rem 0 0">{entry.focus}</p>
		{/if}
		<p class="muted meta">
			{#if body?.load}<span>{body.load}</span>{/if}
			{#if body?.duration_minutes}<span>· about {body.duration_minutes} min</span>{/if}
			<span>· {blocks.length} block{blocks.length === 1 ? '' : 's'}</span>
		</p>
		<div class="actions">
			<button class="ghost" onclick={toggleDone} disabled={busy}>
				{entry.completed_at ? 'Done — undo' : 'Mark done'}
			</button>
			<a class="ghost button-like" href={`/calendar?edit=${entry.id}&on=${entry.scheduled_on}`}>
				Edit
			</a>
			<a class="ghost button-like" href="/calendar">Back to the calendar</a>
		</div>
	</header>

	{#each rehab as protocol (protocol.slug)}
		<section class="panel rehab">
			<p class="eyebrow" style="margin:0 0 0.3rem">Rehab — this comes first</p>
			<strong class="item-title">{protocol.title}</strong>
			<ol class="steps">
				{#each protocol.steps as step, i (i)}
					<li>{step}</li>
				{/each}
			</ol>
			{#if protocol.avoid_while?.length}
				<p class="muted small">Not while: {protocol.avoid_while.join(', ')}</p>
			{/if}
			{#if protocol.see_clinician}
				<p class="muted small">{protocol.see_clinician}</p>
			{/if}
		</section>
	{/each}

	{#each phases as phase, index (phase.key)}
		<section class="phase" class:empty={phase.empty}>
			<div class="phase-head">
				<span class="ordinal">{index + 1}</span>
				<div>
					<h2 class="item-title" style="margin:0">{phase.label}</h2>
					<p class="muted small" style="margin:0.15rem 0 0">{phase.note}</p>
				</div>
			</div>

			{#each phase.items as protocol (protocol.slug)}
				<div class="protocol">
					<strong class="protocol-title">{protocol.title}</strong>
					<ol class="steps">
						{#each protocol.steps as step, i (i)}
							<li>{step}</li>
						{/each}
					</ol>
				</div>
			{/each}

			{#if phase.sets.length}
				<table>
					<tbody>
						{#each phase.sets as block, i (i)}
							<tr>
								<td class="movement">
									{nameOf(block.exercise_slug)}
									{#if block.intent && phase.key === 'training'}
										<span class="chip">{INTENTS[block.intent] ?? block.intent}</span>
									{/if}
								</td>
								<td class="mono prescription">{block.sets} × {block.prescription}</td>
								<td class="mono muted rest">
									{block.rest_seconds ? `${block.rest_seconds}s rest` : ''}
								</td>
							</tr>
							{#if block.intensity || block.tempo || block.notes || block.progression}
								<tr>
									<td colspan="3" class="detail muted">
										{#if block.intensity}<span class="mono">{block.intensity}</span>{/if}
										{#if block.tempo}<span class="mono">tempo {block.tempo}</span>{/if}
										{#if block.notes}<span>{block.notes}</span>{/if}
										{#if block.progression}<span class="next">{block.progression}</span>{/if}
									</td>
								</tr>
							{/if}
						{/each}
					</tbody>
				</table>
			{/if}

			{#if phase.text}
				<p class="cooldown-text">{phase.text}</p>
			{/if}

			{#if phase.empty}
				<p class="muted small nothing">
					Nothing was saved for this phase. Sessions written before the warm-up was
					split into phases carry it as one block — the note above is the instruction.
				</p>
			{/if}
		</section>
	{/each}
{:else if !error}
	<p class="muted">Loading the session…</p>
{/if}

<style>
	.head {
		margin-bottom: 1.4rem;
	}
	.meta {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem;
		font-size: 0.85rem;
		margin: 0.4rem 0 0;
	}
	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		margin-top: 0.8rem;
	}
	.button-like {
		display: inline-flex;
		align-items: center;
		text-decoration: none;
	}
	.rehab {
		border-left: 3px solid var(--warn, #b45309);
		margin-bottom: 1.2rem;
	}
	.phase {
		border-top: 1px solid var(--line);
		padding: 1.1rem 0 0.2rem;
	}
	.phase.empty {
		opacity: 0.65;
	}
	.phase-head {
		display: flex;
		gap: 0.75rem;
		align-items: baseline;
		margin-bottom: 0.7rem;
	}
	.ordinal {
		font-family: var(--mono);
		font-size: 0.8rem;
		opacity: 0.55;
		min-width: 1.2rem;
	}
	.protocol {
		margin: 0 0 0.9rem 1.95rem;
	}
	.protocol-title {
		font-size: 0.9rem;
	}
	.steps {
		margin: 0.3rem 0 0;
		padding-left: 1.1rem;
		line-height: 1.55;
	}
	.steps li {
		margin: 0.15rem 0;
	}
	table {
		width: 100%;
		border-collapse: collapse;
		margin-left: 1.95rem;
		width: calc(100% - 1.95rem);
	}
	.movement {
		padding: 0.35rem 0.6rem 0.35rem 0;
	}
	.prescription {
		white-space: nowrap;
		padding: 0.35rem 0.6rem 0.35rem 0;
	}
	.rest {
		text-align: right;
		white-space: nowrap;
		font-size: 0.8rem;
	}
	.detail {
		padding: 0 0 0.5rem;
		font-size: 0.82rem;
	}
	.detail span + span::before {
		content: ' · ';
	}
	.next {
		opacity: 0.8;
	}
	.cooldown-text,
	.nothing {
		margin: 0 0 0.6rem 1.95rem;
	}
	.small {
		font-size: 0.83rem;
	}
	@media (max-width: 640px) {
		.protocol,
		.cooldown-text,
		.nothing,
		table {
			margin-left: 0;
			width: 100%;
		}
	}
</style>
