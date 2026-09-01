<script>
	import { api } from '$lib/api.js';
	import Failure from '$lib/Failure.svelte';
	import { page } from '$app/state';

	// What the athlete can do, before they have logged it here.
	//
	// The planner sizes every prescription from records, so an athlete who
	// joined yesterday gets the bottom of every ladder — right, given what it
	// knows, and useless to someone who already has twelve pull-ups. This is
	// the shortest form that fixes that: only the figures the planner actually
	// branches on, asked in the units the app measures them in.

	let goal = $state(page.url.searchParams.get('goal') ?? '');
	let questions = $state([]);
	let equipment = $state([]);
	let goalMatched = $state(false);
	let goalName = $state('');

	let bodyweight = $state('');
	let trainsPerWeek = $state('');
	let sleepHours = $state('');
	let owned = $state(new Set());
	// Whether the equipment question has been answered at all. Untouched, it
	// is left out of the save so the server keeps treating it as unanswered.
	let answered = $state(false);
	let answers = $state({});

	let error = $state(null);
	let saved = $state(false);
	let saving = $state(false);
	let loading = $state(true);

	// The goal decides which ladder rungs get asked about, so changing it
	// reloads the questions rather than the whole page.
	async function load() {
		loading = true;
		error = null;
		try {
			const [form, current] = await Promise.all([
				api.get(`/plan/benchmarks?goal=${encodeURIComponent(goal)}`),
				api.get('/baseline')
			]);
			questions = form.questions ?? [];
			equipment = form.equipment ?? [];
			goalMatched = form.goal_matched ?? false;
			goalName = form.goal ?? '';

			bodyweight = current.bodyweight_kg ?? '';
			trainsPerWeek = current.trains_per_week ?? '';
			sleepHours = current.sleep_hours ?? '';
			// null means the question has never been answered, which is not the
			// same as answering "none of it".
			owned = new Set(current.equipment ?? []);
			answered = current.equipment !== null && current.equipment !== undefined;

			const existing = {};
			for (const record of current.records ?? []) {
				existing[record.exercise_slug] =
					record.hold_seconds ?? record.added_kg ?? record.reps ?? '';
			}
			answers = existing;
		} catch (e) {
			error = e;
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		load();
	});

	function toggle(key) {
		const next = new Set(owned);
		// "None of the above" is an answer, not an absence of one, so it
		// clears the rest rather than sitting alongside them.
		if (key === 'floor_only') {
			next.clear();
			if (!owned.has('floor_only')) next.add('floor_only');
		} else {
			next.delete('floor_only');
			next.has(key) ? next.delete(key) : next.add(key);
		}
		owned = next;
		answered = true;
	}

	function fieldFor(question) {
		if (question.measure === 'static_hold') return { unit: 'seconds', key: 'hold_seconds' };
		if (question.measure === 'weighted_reps') return { unit: 'added kg', key: 'added_kg' };
		return { unit: 'reps', key: 'reps' };
	}

	async function save() {
		saving = true;
		saved = false;
		error = null;
		try {
			const records = questions.map((question) => {
				const { key } = fieldFor(question);
				const value = answers[question.exercise_slug];
				// An emptied field is a deletion: the server takes a record with
				// nothing in it as "I was wrong about this".
				const entry = { exercise_slug: question.exercise_slug };
				if (value !== '' && value !== null && value !== undefined) entry[key] = Number(value);
				return entry;
			});

			const body = { records };
			if (bodyweight !== '') body.bodyweight_kg = Number(bodyweight);
			if (trainsPerWeek !== '') body.trains_per_week = Number(trainsPerWeek);
			if (sleepHours !== '') body.sleep_hours = Number(sleepHours);
			if (answered) body.equipment = [...owned];

			await api.put('/baseline', body);
			saved = true;
		} catch (e) {
			error = e;
		} finally {
			saving = false;
		}
	}

	let core = $derived(questions.filter((q) => q.scope === 'core'));
	let ladder = $derived(questions.filter((q) => q.scope !== 'core'));
	let filled = $derived(
		questions.filter((q) => {
			const value = answers[q.exercise_slug];
			return value !== '' && value !== null && value !== undefined;
		}).length
	);
</script>

<p class="eyebrow" style="margin-top:2rem">Your numbers</p>
<h1>Baseline</h1>
<p class="muted column" style="margin-top:0.6rem">
	The planner works from records, so with nothing logged it starts everyone at the bottom of every
	ladder. This is how you tell it where you actually are. Fill in what you know and leave the rest
	blank — a blank is an honest answer and the plan handles it. Anything you log later overrides
	what you put here.
</p>
<div class="bar"></div>

<Failure {error} />

<div class="row form-width">
	<div>
		<label for="bodyweight">Bodyweight (kg)</label>
		<input id="bodyweight" type="number" min="20" max="300" step="0.5" bind:value={bodyweight} />
	</div>
	<div>
		<label for="trains">Sessions a week</label>
		<input id="trains" type="number" min="0" max="14" bind:value={trainsPerWeek} />
	</div>
	<div>
		<label for="sleep">Sleep (hours)</label>
		<input id="sleep" type="number" min="3" max="14" step="0.5" bind:value={sleepHours} />
	</div>
</div>
<p class="muted" style="font-size:0.85rem;margin:0.4rem 0 0;max-width:44rem">
	Bodyweight is what turns a weighted set into a percentage — without it the plan can only say "add
	a little". Sessions a week sets the starting volume before you have a log here. Sleep under about
	eight hours raises injury risk by roughly a third, so the plan adds volume more slowly when it is
	short.
</p>

<div class="bar"></div>
<p class="eyebrow">What you can train on</p>
<p class="muted" style="font-size:0.85rem;margin:0.3rem 0 0.7rem;max-width:44rem">
	The plan will not prescribe a movement you have nothing to perform it on. Leave this untouched and
	it assumes you have the usual bar and bars.
</p>
<div class="grid">
	{#each equipment as item (item.key)}
		<label
			class="panel"
			style="display:flex;gap:0.6rem;align-items:flex-start;text-transform:none;letter-spacing:0;cursor:pointer"
		>
			<input
				type="checkbox"
				checked={owned.has(item.key)}
				onchange={() => toggle(item.key)}
				style="width:auto;margin-top:0.2rem"
			/>
			<span>
				<span style="font-weight:600">{item.label}</span>
				<span class="muted" style="display:block;font-size:0.82rem">{item.note}</span>
			</span>
		</label>
	{/each}
</div>

<div class="bar"></div>
<div class="row form-width" style="align-items:flex-end">
	<div style="flex:2 1 240px">
		<label for="goal">Working toward</label>
		<input id="goal" bind:value={goal} placeholder="Front lever, muscle-up, pistol squat…" />
	</div>
	<button style="flex:0 0 auto" onclick={load} disabled={loading}>
		{loading ? 'Loading' : 'Ask about this goal'}
	</button>
</div>
<p class="muted" style="font-size:0.85rem;margin:0.4rem 0 0;max-width:44rem">
	{#if goalMatched}
		Naming a goal adds the rungs of its ladder below, because those are what decide where you start.
		Currently asking for <strong>{goalName}</strong>.
	{:else}
		Name a goal and the form also asks about the rungs of its ladder, which is what actually places
		you on it.
	{/if}
</p>

{#if core.length}
	<div class="bar"></div>
	<p class="eyebrow">The eight that decide everything else</p>
	<p class="muted" style="font-size:0.85rem;margin:0.3rem 0 0">
		{filled} of {questions.length} answered. Every one of these changes something the plan does.
	</p>

	<table style="margin-top:0.8rem">
		<thead>
			<tr><th>Test</th><th style="width:9rem">Your best</th><th>What it decides</th></tr>
		</thead>
		<tbody>
			{#each core as question (question.exercise_slug)}
				<tr>
					<td>
						<label for={question.exercise_slug} style="text-transform:none;letter-spacing:0">
							{question.prompt}
						</label>
					</td>
					<td>
						<input
							id={question.exercise_slug}
							type="number"
							min="0"
							step={question.measure === 'weighted_reps' ? '1.25' : '1'}
							bind:value={answers[question.exercise_slug]}
							placeholder={fieldFor(question).unit}
						/>
					</td>
					<td class="muted" style="font-size:0.85rem">{question.why}</td>
				</tr>
			{/each}
		</tbody>
	</table>
{/if}

{#if ladder.length}
	<p class="eyebrow" style="margin-top:1.6rem">The ladder to {goalName}</p>
	<p class="muted" style="font-size:0.85rem;margin:0.3rem 0 0">
		Fill in the ones you can hold or hit. The first rung you cannot is roughly where the plan will
		start you.
	</p>

	<table style="margin-top:0.8rem">
		<thead>
			<tr><th>Rung</th><th style="width:9rem">Your best</th><th>Cleared at</th></tr>
		</thead>
		<tbody>
			{#each ladder as question (question.exercise_slug)}
				<tr>
					<td>
						<label for={question.exercise_slug} style="text-transform:none;letter-spacing:0">
							{question.prompt}
						</label>
					</td>
					<td>
						<input
							id={question.exercise_slug}
							type="number"
							min="0"
							step={question.measure === 'weighted_reps' ? '1.25' : '1'}
							bind:value={answers[question.exercise_slug]}
							placeholder={fieldFor(question).unit}
						/>
					</td>
					<td class="muted" style="font-size:0.85rem">{question.why}</td>
				</tr>
			{/each}
		</tbody>
	</table>
{/if}

<div class="row form-width" style="margin-top:1.4rem;align-items:center">
	<button style="flex:0 0 auto" onclick={save} disabled={saving || loading}>
		{saving ? 'Saving' : 'Save my baseline'}
	</button>
	{#if saved}
		<p class="muted" style="margin:0;flex:1 1 auto">
			Saved. <a href={`/plan?goal=${encodeURIComponent(goal)}`}>Build a plan from it</a>.
		</p>
	{:else}
		<p class="muted" style="margin:0;font-size:0.85rem;flex:1 1 200px">
			Used by every plan from now on, until you log the movement for real.
		</p>
	{/if}
</div>
