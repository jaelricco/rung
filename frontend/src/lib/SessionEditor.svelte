<script>
	// One session, being written rather than read. The same shape SessionDetail
	// renders, so anything built here — a routine day, a session typed straight
	// onto the calendar — looks like a session everywhere else in the app.
	let { body, exercises = [] } = $props();

	const INTENTS = [
		['prep', 'Prep'],
		['skill', 'Skill'],
		['strength', 'Strength'],
		['accessory', 'Accessory'],
		['conditioning', 'Conditioning']
	];

	let byCategory = $derived(
		exercises.reduce((acc, e) => {
			(acc[e.category] ??= []).push(e);
			return acc;
		}, {})
	);

	function measureFor(slug) {
		return exercises.find((e) => e.slug === slug)?.measure ?? 'reps';
	}

	// A sensible first prescription for the movement, so the common case is a
	// number to adjust rather than an empty box to fill.
	function defaultPrescription(slug) {
		switch (measureFor(slug)) {
			case 'static_hold':
				return '15s hold';
			case 'weighted_reps':
				return '5 reps + 10kg';
			case 'skill_attempt':
				return '5 attempts';
			default:
				return '8 reps';
		}
	}

	function addBlock() {
		const previous = body.blocks?.[body.blocks.length - 1];
		const slug = previous?.exercise_slug ?? exercises[0]?.slug ?? '';
		body.blocks = [
			...(body.blocks ?? []),
			{
				exercise_slug: slug,
				intent: previous?.intent ?? 'strength',
				sets: 3,
				prescription: defaultPrescription(slug),
				intensity: '',
				rest_seconds: previous?.rest_seconds ?? 120,
				notes: ''
			}
		];
	}

	function removeBlock(index) {
		body.blocks = body.blocks.filter((_, i) => i !== index);
	}

	// Order is not decoration: skill work belongs before the strength work
	// that would fatigue it, so the blocks have to be movable.
	function move(index, delta) {
		const next = index + delta;
		if (next < 0 || next >= body.blocks.length) return;
		const blocks = [...body.blocks];
		[blocks[index], blocks[next]] = [blocks[next], blocks[index]];
		body.blocks = blocks;
	}

	function onExerciseChange(block) {
		block.prescription = defaultPrescription(block.exercise_slug);
	}
</script>

<div class="row">
	<div style="flex:2 1 220px">
		<label for="session-title">Name</label>
		<input id="session-title" bind:value={body.title} placeholder="Push day" maxlength="120" />
	</div>
	<div style="flex:2 1 220px">
		<label for="session-focus">Focus</label>
		<input id="session-focus" bind:value={body.focus} placeholder="planche, straight-arm" maxlength="500" />
	</div>
	<div style="flex:0 1 120px">
		<label for="session-length">Minutes</label>
		<input id="session-length" type="number" min="0" max="600" bind:value={body.duration_minutes} />
	</div>
</div>

{#each body.blocks ?? [] as block, index (index)}
	<div class="block">
		<div class="row">
			<div style="flex:2 1 200px">
				<label for={`block-ex-${index}`}>Exercise</label>
				<select
					id={`block-ex-${index}`}
					bind:value={block.exercise_slug}
					onchange={() => onExerciseChange(block)}
				>
					{#each Object.entries(byCategory) as [category, list] (category)}
						<optgroup label={category}>
							{#each list as exercise (exercise.slug)}
								<option value={exercise.slug}>{exercise.name}</option>
							{/each}
						</optgroup>
					{/each}
				</select>
			</div>
			<div style="flex:1 1 130px">
				<label for={`block-intent-${index}`}>Intent</label>
				<select id={`block-intent-${index}`} bind:value={block.intent}>
					{#each INTENTS as [value, name] (value)}
						<option {value}>{name}</option>
					{/each}
				</select>
			</div>
			<div style="flex:0 1 80px">
				<label for={`block-sets-${index}`}>Sets</label>
				<input id={`block-sets-${index}`} type="number" min="1" max="20" bind:value={block.sets} />
			</div>
			<div style="flex:1 1 140px">
				<label for={`block-pres-${index}`}>Each set</label>
				<input
					id={`block-pres-${index}`}
					bind:value={block.prescription}
					placeholder="12s hold"
					maxlength="500"
				/>
			</div>
			<div style="flex:0 1 100px">
				<label for={`block-rest-${index}`}>Rest (s)</label>
				<input
					id={`block-rest-${index}`}
					type="number"
					min="0"
					max="3600"
					step="15"
					bind:value={block.rest_seconds}
				/>
			</div>
		</div>
		<div class="row" style="margin-top:0.4rem;align-items:flex-end">
			<div style="flex:3 1 260px">
				<label for={`block-notes-${index}`}>Note</label>
				<input
					id={`block-notes-${index}`}
					bind:value={block.notes}
					placeholder="stop the set when the line breaks"
					maxlength="500"
				/>
			</div>
			<div class="block-tools">
				<button class="ghost" type="button" onclick={() => move(index, -1)} disabled={index === 0}>
					↑
				</button>
				<button
					class="ghost"
					type="button"
					onclick={() => move(index, 1)}
					disabled={index === (body.blocks?.length ?? 0) - 1}
				>
					↓
				</button>
				<button class="ghost" type="button" onclick={() => removeBlock(index)}>Remove</button>
			</div>
		</div>
	</div>
{/each}

<button class="ghost" type="button" style="margin-top:0.6rem" onclick={addBlock}>Add exercise</button>

<style>
	.block {
		border-left: 2px solid var(--line);
		padding: 0.6rem 0 0.6rem 0.7rem;
		margin-top: 0.7rem;
	}
	.block-tools {
		display: flex;
		gap: 0.4rem;
		flex: 0 0 auto;
	}
	.block-tools button {
		padding: 0.5rem 0.7rem;
		font-size: 0.8rem;
	}
</style>
