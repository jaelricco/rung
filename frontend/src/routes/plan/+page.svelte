<script>
	import { api } from '$lib/api.js';
	import AiProgress from '$lib/AiProgress.svelte';

	let skill = $state('');
	let weeks = $state(8);
	let daysPerWeek = $state(3);
	let notes = $state('');
	let save = $state(true);

	let plan = $state(null);
	let saved = $state(false);
	let error = $state('');
	let busy = $state(false);
	let progress = $state(null);

	const DAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];

	async function generate() {
		error = '';
		plan = null;
		busy = true;
		progress = { label: 'Starting', percent: 0 };
		try {
			const result = await api.stream(
				'/ai/skill-plan',
				{
					skill,
					weeks: Number(weeks),
					days_per_week: Number(daysPerWeek),
					notes,
					save
				},
				(update) => (progress = update)
			);
			plan = result.plan;
			saved = result.saved;
		} catch (e) {
			error = e.message;
		} finally {
			busy = false;
			progress = null;
		}
	}

	let byWeek = $derived(
		(plan?.sessions ?? []).reduce((acc, s) => {
			(acc[s.week] ??= []).push(s);
			return acc;
		}, {})
	);
</script>

<p class="eyebrow" style="margin-top:2rem">Programming</p>
<h1>Build a plan</h1>
<p class="muted" style="margin-top:0.6rem;max-width:52ch">
	The plan is written against your logged records and any open injury, using only exercises in the
	library.
</p>
<div class="bar"></div>

<div class="row">
	<div style="flex:2 1 240px">
		<label for="skill">Goal</label>
		<input id="skill" bind:value={skill} placeholder="Full front lever, 20 kg weighted pull-up…" />
	</div>
	<div>
		<label for="weeks">Weeks</label>
		<input id="weeks" type="number" min="1" max="24" bind:value={weeks} />
	</div>
	<div>
		<label for="days">Days per week</label>
		<input id="days" type="number" min="1" max="7" bind:value={daysPerWeek} />
	</div>
</div>

<div class="field" style="margin-top:0.6rem">
	<label for="notes">Anything else the plan should account for</label>
	<textarea id="notes" rows="2" bind:value={notes} placeholder="Equipment, schedule, past problems"></textarea>
</div>

<label style="display:flex;align-items:center;gap:0.5rem;text-transform:none;letter-spacing:0">
	<input type="checkbox" bind:checked={save} style="width:auto" />
	Add the sessions to my calendar
</label>

{#if error}
	<div class="notice error" style="margin-top:1rem">{error}</div>
{/if}

<button style="margin-top:1rem" onclick={generate} disabled={busy || !skill.trim()}>
	{busy ? 'Writing your plan' : 'Generate plan'}
</button>

{#if busy && progress}
	<div style="margin-top:1rem">
		<AiProgress
			label={progress.label}
			percent={progress.percent}
			detail={progress.detail}
			done={progress.done}
			total={progress.total}
		/>
	</div>
{/if}

{#if plan}
	<div class="bar"></div>
	<h2>{plan.title}</h2>
	<p class="prose" style="margin-top:0.6rem">{plan.summary}</p>

	{#if plan.restrictions?.length}
		<div class="notice" style="margin-top:0.9rem">
			<strong>Adjusted for:</strong>
			<ul style="margin:0.4rem 0 0;padding-left:1.1rem">
				{#each plan.restrictions as restriction}
					<li>{restriction}</li>
				{/each}
			</ul>
		</div>
	{/if}

	{#if saved}
		<p class="muted" style="font-size:0.85rem;margin-top:0.7rem">Added to your calendar.</p>
	{/if}

	{#each Object.entries(byWeek) as [week, sessions] (week)}
		<div class="bar"></div>
		<p class="eyebrow">Week {week}</p>
		<div style="display:grid;gap:0.7rem;margin-top:0.6rem">
			{#each sessions as session}
				<div class="panel">
					<p class="eyebrow">{DAYS[(session.day_of_week ?? 1) - 1] ?? '—'}</p>
					<p style="font-weight:600;margin:0.15rem 0 0.1rem">{session.title}</p>
					<p class="muted" style="font-size:0.85rem;margin:0 0 0.6rem">{session.focus}</p>

					{#if session.warmup_protocols?.length}
						<p class="mono muted" style="font-size:0.78rem;margin:0 0 0.5rem">
							Warm-up: {session.warmup_protocols.join(', ')}
						</p>
					{/if}

					<table>
						<tbody>
							{#each session.blocks ?? [] as block}
								<tr>
									<td style="width:38%">{block.exercise_slug}</td>
									<td class="mono">{block.sets} × {block.prescription}</td>
									<td class="mono muted" style="width:22%">
										{block.rest_seconds ? `${block.rest_seconds}s rest` : ''}
									</td>
								</tr>
								{#if block.notes}
									<tr>
										<td colspan="3" class="muted" style="font-size:0.82rem;padding-top:0">
											{block.notes}
										</td>
									</tr>
								{/if}
							{/each}
						</tbody>
					</table>
				</div>
			{/each}
		</div>
	{/each}
{/if}
