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
	let skills = $state([]);
	let ceiling = $state(5);
	let goalMatched = $state(false);
	let goalName = $state('');

	let bodyweight = $state('');
	let trainsPerWeek = $state('');
	let sleepHours = $state('');
	let owned = $state(new Set());
	// Whether the equipment question has been answered at all. Untouched, it
	// is left out of the save so the server keeps treating it as unanswered.
	let answered = $state(false);
	// Which skills are in flight. Every one of them spends from the same
	// tendon budget, which is why this is a question and not a guess.
	let learning = $state(new Set());
	let learningAnswered = $state(false);
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
			const [form, current, catalogue] = await Promise.all([
				api.get(`/plan/benchmarks?goal=${encodeURIComponent(goal)}`),
				api.get('/baseline'),
				api.get('/skills')
			]);
			questions = form.questions ?? [];
			equipment = form.equipment ?? [];
			// Balanced strength is a fallback track rather than something you
			// set out to learn, so it is not offered here.
			skills = (catalogue.skills ?? []).filter((s) => !s.foundation);
			ceiling = catalogue.tendon_ceiling ?? 5;
			goalMatched = form.goal_matched ?? false;
			goalName = form.goal ?? '';

			bodyweight = current.bodyweight_kg ?? '';
			trainsPerWeek = current.trains_per_week ?? '';
			sleepHours = current.sleep_hours ?? '';
			// null means the question has never been answered, which is not the
			// same as answering "none of it".
			owned = new Set(current.equipment ?? []);
			answered = current.equipment !== null && current.equipment !== undefined;
			learning = new Set(current.learning ?? []);
			learningAnswered = current.learning !== null && current.learning !== undefined;

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

	function toggleLearning(key) {
		const next = new Set(learning);
		next.has(key) ? next.delete(key) : next.add(key);
		learning = next;
		learningAnswered = true;
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
			if (learningAnswered) body.learning = [...learning];

			await api.put('/baseline', body);
			saved = true;
		} catch (e) {
			error = e;
		} finally {
			saving = false;
		}
	}

	// The budget, computed here rather than waiting for the plan to say it:
	// the moment to find out that a fourth maximal skill is one too many is
	// while you are picking it.
	let spent = $derived(
		[...learning].reduce((total, key) => total + (skills.find((s) => s.key === key)?.cost ?? 1), 0)
	);
	let overBudget = $derived(spent > ceiling);

	// Grouped by what each one costs, because that is the thing this section
	// is trying to teach.
	const TIERS = [
		{ cost: 3, label: 'Maximal', note: 'Three units each. Two of these is already a full week.' },
		{ cost: 2, label: 'Demanding', note: 'Two units each.' },
		{ cost: 1, label: 'Everything else', note: 'One unit each.' }
	];
	let tiers = $derived(
		TIERS.map((tier) => ({
			...tier,
			skills: skills
				.filter((s) => (s.cost || 1) === tier.cost)
				.sort((a, b) => a.name.localeCompare(b.name))
		})).filter((tier) => tier.skills.length)
	);

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
<p class="lede" style="margin:0.4rem 0 0;max-width:44rem">
	Bodyweight is what turns a weighted set into a percentage — without it the plan can only say "add
	a little". Sessions a week sets the starting volume before you have a log here. Sleep under about
	eight hours raises injury risk by roughly a third, so the plan adds volume more slowly when it is
	short.
</p>

<div class="bar"></div>
<p class="eyebrow">What you can train on</p>
<p class="lede" style="margin:0.3rem 0 0.7rem;max-width:44rem">
	The plan will not prescribe a movement you have nothing to perform it on. Leave this untouched and
	it assumes you have the usual bar and bars.
</p>
<div class="grid roomy">
	{#each equipment as item (item.key)}
		<label
			class="choice panel"
			style="display:flex;gap:0.6rem;align-items:flex-start"
		>
			<input
				type="checkbox"
				checked={owned.has(item.key)}
				onchange={() => toggle(item.key)}
				style="width:auto;margin-top:0.2rem"
			/>
			<span>
				<span class="item-title">{item.label}</span>
				<span class="note" style="display:block">{item.note}</span>
			</span>
		</label>
	{/each}
</div>

<div class="bar"></div>
<p class="eyebrow">What you are learning right now</p>
<p class="muted" style="font-size:0.85rem;margin:0.3rem 0 0.9rem;max-width:44rem">
	Every maximal straight-arm skill spends from the same tendons, and they do not know which goal a
	set belonged to. Naming what is already in flight is what lets a plan for a new skill say what to
	park instead of quietly adding a fourth. Leave this untouched if you would rather it did not.
</p>

<div class="panel form-width" style="margin-bottom:1rem">
	<div style="display:flex;align-items:baseline;gap:0.6rem;flex-wrap:wrap">
		<strong style="font-size:1.05rem">{spent} of {ceiling}</strong>
		<span class="muted" style="font-size:0.85rem">units of maximal straight-arm work a week</span>
	</div>

	<!-- One segment per unit the week holds, then overflow in the warning
	     colour: the shape of being over budget, not just the number. -->
	<div style="display:flex;gap:3px;margin-top:0.6rem">
		{#each Array(Math.max(ceiling, spent)) as _, i (i)}
			<span
				style="flex:1 1 0;height:8px;border-radius:2px;background:{i < spent
					? i < ceiling
						? 'var(--signal)'
						: 'var(--bad)'
					: 'var(--line)'}"
			></span>
		{/each}
	</div>

	<p class="muted" style="font-size:0.85rem;margin:0.7rem 0 0">
		{#if !learning.size}
			Nothing selected. The plan will assume whatever you ask it for is the only thing you are
			training.
		{:else if overBudget}
			<span style="color:var(--bad);font-weight:600">That is more than one athlete recovers from.</span
			>
			A plan for any of these will name which to park — cheapest first, and never the skill the
			others are built on. Adding a skill without removing one is the shape almost every
			straight-arm injury has.
		{:else if spent === ceiling}
			Full. Anything you add from here will push a plan into telling you what to drop.
		{:else}
			Inside what a week holds.
		{/if}
	</p>
</div>

{#if !loading && !skills.length}
	<div class="notice form-width">
		The skill catalogue came back empty, so there is nothing to pick from. Everything else on this
		page still saves — this section is the only part affected.
	</div>
{/if}

{#each tiers as tier (tier.cost)}
	<p class="eyebrow" style="margin-top:1rem">{tier.label}</p>
	<p class="muted" style="font-size:0.82rem;margin:0.2rem 0 0.5rem">{tier.note}</p>
	<div class="grid">
		{#each tier.skills as skill (skill.key)}
			<label
				class="panel"
				style="display:flex;gap:0.6rem;align-items:flex-start;text-transform:none;letter-spacing:0;cursor:pointer"
			>
				<input
					type="checkbox"
					checked={learning.has(skill.key)}
					onchange={() => toggleLearning(skill.key)}
					style="width:auto;margin-top:0.25rem"
				/>
				<span>
					<span style="font-weight:600">{skill.name}</span>
					<span class="muted" style="font-size:0.78rem"> · {skill.cost} unit{skill.cost === 1 ? '' : 's'}</span>
					{#if skill.frequency}
						<span class="muted" style="display:block;font-size:0.8rem">{skill.frequency}</span>
					{/if}
				</span>
			</label>
		{/each}
	</div>
{/each}

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
<p class="lede" style="margin:0.4rem 0 0;max-width:44rem">
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
	<p class="lede" style="margin:0.3rem 0 0">
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
						<label class="choice" for={question.exercise_slug}>
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
					<td class="note">{question.why}</td>
				</tr>
			{/each}
		</tbody>
	</table>
{/if}

{#if ladder.length}
	<p class="eyebrow" style="margin-top:1.6rem">The ladder to {goalName}</p>
	<p class="lede" style="margin:0.3rem 0 0">
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
						<label class="choice" for={question.exercise_slug}>
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
					<td class="note">{question.why}</td>
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
		<p class="lede" style="margin:0;flex:1 1 200px">
			Used by every plan from now on, until you log the movement for real.
		</p>
	{/if}
</div>
