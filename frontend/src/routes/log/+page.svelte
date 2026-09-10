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
			// A weighted hold is both numbers. Either on its own is a
			// different exercise, and the server refuses the set without both.
			if (kind === 'weighted_hold') {
				out.hold_seconds = Number(s.hold_seconds);
				out.weight_kg = Number(s.weight_kg);
			}
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
	<!-- Every binding below writes through sets[index] rather than through the
	     each-item value. It reads as the long way round, and it is the only way
	     that works here: this each block is the sole child of its container, so
	     Svelte compiles it as a controlled block, and under the dev build the
	     per-item source it hands the binding is already gone by the time the
	     setter runs — changing the exercise threw and the set kept whatever it
	     was created with. Indexing the array touches the one signal that is
	     always live, and behaves identically in the production build, which
	     never had the fault. The item is named _set to keep it that way. -->
	{#each sets as _set, index (index)}
		{@const measure = measureFor(sets[index].exercise_slug)}
		<div class="panel">
			<div class="row">
				<div style="flex:2 1 220px">
					<label for={`ex-${index}`}>Exercise</label>
					<select id={`ex-${index}`} bind:value={sets[index].exercise_slug}>
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
						<input id={`reps-${index}`} type="number" min="0" bind:value={sets[index].reps} />
					</div>
				{/if}

				{#if measure === 'weighted_reps' || measure === 'weighted_hold'}
					<div>
						<label for={`kg-${index}`}>Added kg</label>
						<input id={`kg-${index}`} type="number" step="0.5" bind:value={sets[index].weight_kg} />
					</div>
				{/if}

				{#if measure === 'static_hold' || measure === 'weighted_hold'}
					<div>
						<label for={`hold-${index}`}>Hold (s)</label>
						<input id={`hold-${index}`} type="number" step="0.5" min="0" bind:value={sets[index].hold_seconds} />
					</div>
				{/if}

				{#if measure === 'skill_attempt'}
					<div>
						<label for={`made-${index}`}>Result</label>
						<select id={`made-${index}`} bind:value={sets[index].success}>
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
