<script>
	import { goto } from '$app/navigation';
	import { api } from '$lib/api.js';

	let exercises = $state([]);
	let sets = $state([]);
	let notes = $state('');
	let rpe = $state('');
	let error = $state('');
	let busy = $state(false);

	$effect(() => {
		api.get('/exercises')
			.then((list) => {
				exercises = list;
				if (sets.length === 0) addSet();
			})
			.catch((e) => (error = e.message));
	});

	function measureFor(slug) {
		return exercises.find((e) => e.slug === slug)?.measure ?? 'reps';
	}

	function addSet() {
		const previous = sets[sets.length - 1];
		const slug = previous?.exercise_slug ?? exercises[0]?.slug ?? '';
		sets = [
			...sets,
			{
				exercise_slug: slug,
				reps: previous?.reps ?? '',
				weight_kg: previous?.weight_kg ?? '',
				hold_seconds: previous?.hold_seconds ?? '',
				success: true
			}
		];
	}

	function removeSet(index) {
		sets = sets.filter((_, i) => i !== index);
	}

	function toPayload() {
		return sets.map((s) => {
			const kind = measureFor(s.exercise_slug);
			const out = { exercise_slug: s.exercise_slug, kind };
			if (kind === 'reps') out.reps = Number(s.reps);
			if (kind === 'weighted_reps') {
				out.reps = Number(s.reps);
				out.weight_kg = Number(s.weight_kg);
			}
			if (kind === 'static_hold') out.hold_seconds = Number(s.hold_seconds);
			if (kind === 'skill_attempt') out.success = Boolean(s.success);
			return out;
		});
	}

	async function save() {
		error = '';
		busy = true;
		try {
			await api.post('/workouts', {
				notes,
				rpe: rpe === '' ? null : Number(rpe),
				sets: toPayload()
			});
			goto('/');
		} catch (e) {
			error = e.message;
		} finally {
			busy = false;
		}
	}

	let byCategory = $derived(
		exercises.reduce((acc, e) => {
			(acc[e.category] ??= []).push(e);
			return acc;
		}, {})
	);
</script>

<p class="eyebrow" style="margin-top:2rem">New entry</p>
<h1>Log a session</h1>
<div class="bar"></div>

{#if error}
	<div class="notice error" style="margin-bottom:1rem">{error}</div>
{/if}

<div class="form-width" style="display:grid;gap:0.7rem">
	{#each sets as set, index (index)}
		{@const measure = measureFor(set.exercise_slug)}
		<div class="panel">
			<div class="row">
				<div style="flex:2 1 220px">
					<label for={`ex-${index}`}>Exercise</label>
					<select id={`ex-${index}`} bind:value={set.exercise_slug}>
						{#each Object.entries(byCategory) as [category, list] (category)}
							<optgroup label={category}>
								{#each list as exercise (exercise.slug)}
									<option value={exercise.slug}>{exercise.name}</option>
								{/each}
							</optgroup>
						{/each}
					</select>
				</div>

				{#if measure === 'reps' || measure === 'weighted_reps'}
					<div>
						<label for={`reps-${index}`}>Reps</label>
						<input id={`reps-${index}`} type="number" min="0" bind:value={set.reps} />
					</div>
				{/if}

				{#if measure === 'weighted_reps'}
					<div>
						<label for={`kg-${index}`}>Added kg</label>
						<input id={`kg-${index}`} type="number" step="0.5" bind:value={set.weight_kg} />
					</div>
				{/if}

				{#if measure === 'static_hold'}
					<div>
						<label for={`hold-${index}`}>Hold (s)</label>
						<input id={`hold-${index}`} type="number" step="0.5" min="0" bind:value={set.hold_seconds} />
					</div>
				{/if}

				{#if measure === 'skill_attempt'}
					<div>
						<label for={`made-${index}`}>Result</label>
						<select id={`made-${index}`} bind:value={set.success}>
							<option value={true}>Made</option>
							<option value={false}>Missed</option>
						</select>
					</div>
				{/if}
			</div>

			{#if sets.length > 1}
				<button class="link" style="margin-top:0.5rem" onclick={() => removeSet(index)}>
					Remove set {index + 1}
				</button>
			{/if}
		</div>
	{/each}
</div>

<button class="ghost" style="margin-top:0.8rem" onclick={addSet}>Add another set</button>

<div class="bar"></div>

<div class="row form-width">
	<div style="flex:3 1 260px">
		<label for="notes">Notes</label>
		<textarea id="notes" rows="3" bind:value={notes} placeholder="How it felt, anything that hurt"></textarea>
	</div>
	<div>
		<label for="rpe">Effort (1–10)</label>
		<input id="rpe" type="number" min="1" max="10" bind:value={rpe} />
	</div>
</div>

<button style="margin-top:1rem" onclick={save} disabled={busy || sets.length === 0}>
	{busy ? 'Saving' : 'Save session'}
</button>
