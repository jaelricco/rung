<script>
	import { api } from '$lib/api.js';
	import { session } from '$lib/session.svelte.js';
	import AiProgress from '$lib/AiProgress.svelte';

	const TIERS = ['untested', 'beginner', 'novice', 'intermediate', 'advanced', 'elite'];

	let level = $state(null);
	let workouts = $state([]);
	let error = $state('');
	let review = $state('');
	let reviewBusy = $state(false);
	let reviewProgress = $state(null);

	$effect(() => {
		if (!session.user) return;
		Promise.all([api.get('/level'), api.get('/workouts?limit=8')])
			.then(([lvl, logs]) => {
				level = lvl;
				workouts = logs;
			})
			.catch((e) => (error = e.message));
	});

	async function runReview() {
		reviewBusy = true;
		error = '';
		reviewProgress = { label: 'Starting', percent: 0 };
		try {
			const result = await api.stream('/ai/review', {}, (update) => (reviewProgress = update));
			review = result.text;
		} catch (e) {
			error = e.message;
		} finally {
			reviewBusy = false;
			reviewProgress = null;
		}
	}

	function describeSet(s) {
		if (s.kind === 'reps') return `${s.reps} reps`;
		if (s.kind === 'weighted_reps') return `${s.reps} reps + ${s.weight_kg} kg`;
		if (s.kind === 'static_hold') return `${s.hold_seconds}s hold`;
		return s.success ? 'made' : 'missed';
	}

	function bestOf(record) {
		if (record.best_added_kg != null) return `+${record.best_added_kg} kg`;
		if (record.best_hold_seconds != null) return `${record.best_hold_seconds}s`;
		if (record.best_reps != null) return `${record.best_reps} reps`;
		return '—';
	}

	let trainedRecords = $derived(
		(level?.records ?? []).filter((r) => r.tier && r.tier !== 'untested').slice(0, 12)
	);
</script>

<p class="eyebrow" style="margin-top:2rem">
	{session.user?.display_name}
</p>
<h1>Where you stand</h1>

{#if error}
	<div class="notice error" style="margin-top:1rem">{error}</div>
{/if}

{#if level}
	<div class="bar"></div>
	<h2>Level by category</h2>
	<p class="muted" style="font-size:0.88rem;margin:0.3rem 0 1rem">
		Computed from your logged sets, not estimated.
	</p>

	<div class="grid">
		{#each level.categories as category (category.category)}
			<div class="panel">
				<p class="eyebrow">{category.category}</p>
				<p class="stat" style="margin:0.2rem 0 0">{category.tier}</p>
				<div class="rig">
					{#each TIERS.slice(1) as _, i}
						<span class:on={i < category.tier_rank}></span>
					{/each}
				</div>
				<p class="muted" style="font-size:0.78rem;margin:0.55rem 0 0">
					{category.based_on || 'Nothing logged yet'}
				</p>
			</div>
		{/each}
	</div>

	<div class="bar"></div>
	<h2>Last 28 days</h2>
	<div class="grid" style="margin-top:0.8rem">
		<div class="panel">
			<p class="eyebrow">Sessions this week</p>
			<p class="stat">{level.sessions_last_7_days}</p>
		</div>
		<div class="panel">
			<p class="eyebrow">Sessions, 28 days</p>
			<p class="stat">{level.sessions_last_28_days}</p>
		</div>
		<div class="panel">
			<p class="eyebrow">Sets, 28 days</p>
			<p class="stat">{level.sets_last_28_days}</p>
		</div>
		<div class="panel">
			<p class="eyebrow">Open injuries</p>
			<p class="stat" style:color={level.open_injuries.length ? 'var(--bad)' : 'var(--chalk)'}>
				{level.open_injuries.length}
			</p>
		</div>
	</div>

	{#if trainedRecords.length}
		<div class="bar"></div>
		<h2>Records</h2>
		<table style="margin-top:0.8rem">
			<thead>
				<tr><th>Exercise</th><th>Best</th><th>Tier</th><th>Last trained</th></tr>
			</thead>
			<tbody>
				{#each trainedRecords as record (record.slug)}
					<tr>
						<td>{record.name}</td>
						<td class="mono">{bestOf(record)}</td>
						<td class="mono muted">{record.tier}</td>
						<td class="mono muted">{record.last_trained ?? '—'}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
{/if}

<div class="bar"></div>
<h2>Coach review</h2>
<p class="muted" style="font-size:0.88rem;margin:0.3rem 0 0.9rem">
	Reads your last four weeks and names what to change.
</p>

{#if review}
	<div class="panel prose">{review}</div>
	<button class="ghost" style="margin-top:0.7rem" onclick={runReview} disabled={reviewBusy}>
		{reviewBusy ? 'Reading your log' : 'Run it again'}
	</button>
{:else}
	<button onclick={runReview} disabled={reviewBusy}>
		{reviewBusy ? 'Reading your log' : 'Review my training'}
	</button>
{/if}

{#if reviewBusy && reviewProgress}
	<div style="margin-top:0.8rem">
		<AiProgress
			label={reviewProgress.label}
			percent={reviewProgress.percent}
			detail={reviewProgress.detail}
		/>
	</div>
{/if}

<div class="bar"></div>
<h2>Recent sessions</h2>

{#if workouts.length === 0}
	<div class="empty" style="margin-top:0.8rem">
		Nothing logged yet. <a href="/log">Log your first session</a> and the level readout fills in.
	</div>
{:else}
	<div style="margin-top:0.8rem;display:grid;gap:0.7rem">
		{#each workouts as workout (workout.id)}
			<div class="panel">
				<p class="eyebrow">
					{new Date(workout.performed_at).toLocaleDateString()}
					{#if workout.rpe}· RPE {workout.rpe}{/if}
				</p>
				<div style="margin-top:0.5rem;display:grid;gap:0.2rem">
					{#each workout.sets as set (set.id)}
						<p class="mono" style="margin:0;font-size:0.86rem">
							{set.exercise_name}
							<span class="muted"> — {describeSet(set)}</span>
						</p>
					{/each}
				</div>
				{#if workout.notes}
					<p class="muted" style="font-size:0.85rem;margin:0.6rem 0 0">{workout.notes}</p>
				{/if}
			</div>
		{/each}
	</div>
{/if}
