<script>
	import { api } from '$lib/api.js';
	import AiProgress from '$lib/AiProgress.svelte';
	import Failure from '$lib/Failure.svelte';

	const DISCIPLINES = [
		'', 'weighted', 'statics', 'dynamics', 'streetlifting', 'freestyle', 'endurance', 'mixed'
	];

	let discipline = $state('');
	let country = $state('CH');
	let includeUnconfirmed = $state(false);

	let events = $state([]);
	let report = $state(null);
	let error = $state(null);
	let loading = $state(false);
	let searching = $state(false);

	async function load() {
		loading = true;
		error = null;
		try {
			const params = new URLSearchParams();
			if (discipline) params.set('discipline', discipline);
			if (country) params.set('country', country);
			if (includeUnconfirmed) params.set('include_unconfirmed', 'true');
			events = await api.get(`/events?${params}`);
		} catch (e) {
			error = e;
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		load();
	});

	async function search() {
		searching = true;
		error = null;
		report = null;
		try {
			report = await api.post('/events/discover', { discipline, country });
			await load();
		} catch (e) {
			error = e;
		} finally {
			searching = false;
		}
	}

	async function toggleRegister(event) {
		try {
			if (event.registered) {
				await api.del(`/events/${event.id}/register`);
			} else {
				await api.post(`/events/${event.id}/register`, { goal: '' });
			}
			await load();
		} catch (e) {
			error = e;
		}
	}

	async function confirm(event, confirmed) {
		try {
			await api.post(`/events/${event.id}/confirm`, { confirmed, note: '' });
			await load();
		} catch (e) {
			error = e;
		}
	}

	const LABELS = {
		date_confirmed: 'Date checked against source',
		human_confirmed: 'Confirmed by hand',
		source_live: 'Needs confirming',
		unverified: 'Needs confirming'
	};

	function badgeColour(confidence) {
		if (confidence === 'date_confirmed' || confidence === 'human_confirmed') return 'var(--good)';
		return 'var(--signal)';
	}
</script>

<p class="eyebrow" style="margin-top:2rem">Competitions</p>
<h1>Find an event</h1>
<p class="muted" style="margin-top:0.6rem;max-width:58ch">
	Every date here was read off a live page and then checked against that page again by the server.
	The source is shown so you can look yourself.
</p>
<div class="bar"></div>

<div class="row">
	<div>
		<label for="discipline">Discipline</label>
		<select id="discipline" bind:value={discipline}>
			{#each DISCIPLINES as option}
				<option value={option}>{option || 'Any'}</option>
			{/each}
		</select>
	</div>
	<div>
		<label for="country">Country code</label>
		<input id="country" bind:value={country} maxlength="2" placeholder="CH" />
	</div>
</div>

<label style="display:flex;align-items:center;gap:0.5rem;text-transform:none;letter-spacing:0;margin-top:0.7rem">
	<input type="checkbox" bind:checked={includeUnconfirmed} style="width:auto" />
	Also show events that haven't been confirmed
</label>

<div style="display:flex;gap:0.6rem;margin-top:1rem;flex-wrap:wrap">
	<button onclick={search} disabled={searching}>
		{searching ? 'Searching the web' : 'Search for new events'}
	</button>
	<button class="ghost" onclick={load} disabled={loading}>Refresh list</button>
</div>

{#if searching}
	<!-- Discovery runs web search server-side and reports no fraction, so the
	     bar sweeps rather than showing a percentage it would have to invent. -->
	<div style="margin-top:1rem">
		<AiProgress
			label="Searching the web for events, then checking every date against its source"
			percent={null}
		/>
	</div>
{/if}

<Failure {error} />

{#if report}
	<div class="notice" style="margin-top:1rem">
		{#if report.from_cache}
			This region was searched within the last day, so the saved results are shown. Nothing new was
			fetched.
		{:else}
			{report.searches_used} searches ran. {report.confirmed} confirmed against their source,
			{report.needs_review} need confirming, {report.rejected} discarded.
		{/if}
	</div>

	{#if report.rejected > 0}
		<details style="margin-top:0.7rem">
			<summary class="muted" style="cursor:pointer;font-size:0.88rem">
				What was discarded and why
			</summary>
			<div style="margin-top:0.6rem;display:grid;gap:0.4rem">
				{#each report.outcomes.filter((o) => o.confidence === 'rejected') as outcome}
					<div class="panel" style="border-left:2px solid var(--bad)">
						<p style="margin:0;font-size:0.9rem">{outcome.candidate.name}</p>
						<p class="muted" style="margin:0.25rem 0 0;font-size:0.83rem">{outcome.note}</p>
					</div>
				{/each}
			</div>
		</details>
	{/if}
{/if}

<div class="bar"></div>

{#if events.length === 0}
	<div class="empty">
		Nothing found for this filter yet. Search the web above, or widen the country and discipline.
	</div>
{:else}
	<div style="display:grid;gap:0.7rem">
		{#each events as event (event.id)}
			<div class="panel">
				<div style="display:flex;gap:1rem;align-items:baseline;flex-wrap:wrap">
					<p class="mono" style="margin:0;font-size:0.95rem;color:var(--signal)">
						{event.starts_on}{event.ends_on && event.ends_on !== event.starts_on
							? ` – ${event.ends_on}`
							: ''}
					</p>
					<p style="margin:0;font-weight:600">{event.name}</p>
					<span class="spacer" style="margin-left:auto"></span>
					<span class="eyebrow" style="color:{badgeColour(event.confidence)}">
						{LABELS[event.confidence] ?? event.confidence}
					</span>
				</div>

				<p class="muted" style="margin:0.35rem 0 0;font-size:0.87rem">
					{event.discipline} · {event.city}{event.city && event.country ? ', ' : ''}{event.country}
				</p>

				{#if event.evidence}
					<p
						class="mono muted"
						style="margin:0.6rem 0 0;font-size:0.78rem;border-left:2px solid var(--line);padding-left:0.6rem"
					>
						{event.evidence}
					</p>
				{/if}

				{#if event.confidence !== 'date_confirmed' && event.confidence !== 'human_confirmed'}
					<p style="margin:0.6rem 0 0;font-size:0.85rem;color:var(--signal)">{event.check_note}</p>
				{/if}

				<div style="display:flex;gap:0.9rem;margin-top:0.8rem;flex-wrap:wrap;align-items:center">
					{#if event.source_url}
						<a
							href={event.source_url}
							target="_blank"
							rel="noopener noreferrer"
							style="font-size:0.85rem"
						>
							Check the source
						</a>
					{/if}
					{#if event.checked_at}
						<span class="mono muted" style="font-size:0.76rem">last checked {event.checked_at}</span>
					{/if}
					<span style="margin-left:auto"></span>

					{#if event.confidence === 'source_live' || event.confidence === 'unverified'}
						<button class="ghost" onclick={() => confirm(event, true)}>Looks right</button>
						<button class="link" onclick={() => confirm(event, false)}>Wrong</button>
					{/if}

					<button class="ghost" onclick={() => toggleRegister(event)}>
						{event.registered ? 'On my calendar' : 'Add to my calendar'}
					</button>
				</div>
			</div>
		{/each}
	</div>
{/if}
