<script>
	import { goto } from '$app/navigation';
	import { api } from '$lib/api.js';
	import ExercisePicker from '$lib/ExercisePicker.svelte';

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
</script>

<p class="eyebrow" style="margin-top:2rem">New entry</p>
<h1>Log a session</h1>
<div class="bar"></div>

{#if error}
	<div class="notice error" style="margin-bottom:1rem">{error}</div>
{/if}

<div class="form-width" style="display:grid;gap:0.7rem">
	<!-- Every field writes through sets[index] rather than the each alias:
	     binding to the alias here compiled to a write against an item source
	     that was undefined at the time, so what the athlete typed never
	     reached the state and the set was logged as a zero. -->
	{#each sets as set, index (index)}
		{@const measure = measureFor(set.exercise_slug)}
		<div class="panel">
			<div class="row">
				<div style="flex:2 1 220px">
					<label for={`ex-${index}`}>Exercise</label>
					<ExercisePicker id={`ex-${index}`} bind:value={sets[index].exercise_slug} {exercises} />
				</div>

				{#if measure === 'reps' || measure === 'weighted_reps'}
					<div>
						<label for={`reps-${index}`}>Reps</label>
						<input id={`reps-${index}`} type="number" min="0" bind:value={sets[index].reps} />
					</div>
				{/if}

				{#if measure === 'weighted_reps'}
					<div>
						<label for={`kg-${index}`}>Added kg</label>
						<input id={`kg-${index}`} type="number" step="0.5" bind:value={sets[index].weight_kg} />
					</div>
				{/if}

				{#if measure === 'static_hold'}
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
