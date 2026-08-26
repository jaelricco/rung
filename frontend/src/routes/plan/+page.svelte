<script>
	import { api } from '$lib/api.js';
	import AiProgress from '$lib/AiProgress.svelte';
	import SessionDetail from '$lib/SessionDetail.svelte';
	import { DAY_SHORT, weekSlots, sessionShape, isoDate, mondayOf, addDays, formatDate } from '$lib/week.js';

	let skill = $state('');
	let weeks = $state(8);
	let daysPerWeek = $state(3);
	let notes = $state('');
	let research = $state(true);
	let startsOn = $state(isoDate(mondayOf(new Date())));

	let plan = $state(null);
	let warnings = $state([]);
	let error = $state('');
	let busy = $state(false);
	let progress = $state(null);

	let savedPlanId = $state('');
	let saving = $state(false);
	let saveError = $state('');
	let openKey = $state('');

	async function generate() {
		error = '';
		plan = null;
		warnings = [];
		savedPlanId = '';
		busy = true;
		progress = { label: 'Starting', percent: 0 };
		try {
			const result = await api.stream(
				'/ai/skill-plan',
				{
					skill,
					weeks: Number(weeks),
					days_per_week: Number(daysPerWeek),
					starts_on: startsOn,
					notes,
					no_research: !research,
					save: false
				},
				(update) => (progress = update)
			);
			plan = result.plan;
			warnings = result.warnings ?? [];
		} catch (e) {
			error = e.message;
		} finally {
			busy = false;
			progress = null;
		}
	}

	// Generating and scheduling are separate on purpose: a plan is worth
	// reading before it becomes eight weeks of appointments.
	async function addToCalendar() {
		saving = true;
		saveError = '';
		try {
			const result = await api.post('/plans', { plan, goal: skill, starts_on: startsOn });
			savedPlanId = result.plan_id;
		} catch (e) {
			saveError = e.message;
		} finally {
			saving = false;
		}
	}

	// The plan arrives as a flat list of sessions; the week is the unit people
	// actually read it in.
	let byWeek = $derived.by(() => {
		const sessions = plan?.sessions ?? [];
		const weeksInPlan = [...new Set(sessions.map((s) => s.week))].sort((a, b) => a - b);
		// The plan is written in week and day numbers; the calendar it is
		// heading for runs on dates, so map one onto the other from the start
		// date the athlete picked.
		const first = mondayOf(new Date(`${startsOn}T00:00:00`));
		return weeksInPlan.map((week) => {
			const inWeek = sessions.filter((s) => s.week === week);
			const monday = addDays(first, (week - 1) * 7);
			return {
				week,
				monday,
				slots: weekSlots(inWeek, (s) => s.day_of_week).map((slot, day) => ({
					...slot,
					date: addDays(monday, day)
				})),
				sessions: inWeek.length,
				sets: inWeek.reduce(
					(total, s) => total + (s.blocks ?? []).reduce((n, b) => n + (Number(b.sets) || 0), 0),
					0
				),
				deload: inWeek.some((s) => (s.load ?? '').toLowerCase() === 'deload'),
				phase: (plan?.phases ?? []).find((p) => phaseCovers(p, week))
			};
		});
	});

	let openSession = $derived.by(() => {
		for (const week of byWeek) {
			for (const slot of week.slots) {
				for (const [i, session] of slot.items.entries()) {
					if (keyOf(week.week, slot.day, i) === openKey) return { session, week: week.week, slot };
				}
			}
		}
		return null;
	});

	const keyOf = (week, day, i) => `${week}-${day}-${i}`;

	function dayLabel(date) {
		return date.getDate() === 1 ? formatDate(isoDate(date)) : String(date.getDate());
	}

	// Phases name their span as "1-3" or "4", which is the only place the
	// week-to-phase mapping exists.
	function phaseCovers(phase, week) {
		const [from, to] = String(phase?.weeks ?? '')
			.split(/[^0-9]+/)
			.filter(Boolean)
			.map(Number);
		if (!from) return false;
		return week >= from && week <= (to ?? from);
	}
</script>

<p class="eyebrow" style="margin-top:2rem">Programming</p>
<h1>Build a plan</h1>
<p class="muted column" style="margin-top:0.6rem">
	The coach researches how the skill is actually trained, then writes against your logged records
	and any open injury, using only exercises in the library.
</p>
<div class="bar"></div>

<div class="row form-width">
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
	<div>
		<label for="starts">Starts on</label>
		<input id="starts" type="date" bind:value={startsOn} />
	</div>
</div>

<div class="field form-width" style="margin-top:0.6rem">
	<label for="notes">Anything else the plan should account for</label>
	<textarea id="notes" rows="2" bind:value={notes} placeholder="Equipment, schedule, past problems"></textarea>
</div>

<label class="form-width" style="display:flex;align-items:center;gap:0.5rem;text-transform:none;letter-spacing:0">
	<input type="checkbox" bind:checked={research} style="width:auto" />
	Research the skill first. Slower, and the plan knows what the ladder to it looks like.
</label>

{#if error}
	<div class="notice error" style="margin-top:1rem">{error}</div>
{/if}

<button style="margin-top:1rem" onclick={generate} disabled={busy || !skill.trim()}>
	{busy ? 'Writing your plan' : 'Generate plan'}
</button>

{#if busy && progress}
	<div style="margin-top:1rem">
		<!-- Research is a single web-search request that reports nothing until it
		     returns, so the bar sweeps rather than inventing a percentage. -->
		<AiProgress
			label={progress.label}
			percent={progress.indeterminate ? null : progress.percent}
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

	<div class="row form-width" style="margin-top:1rem;align-items:center">
		{#if savedPlanId}
			<p class="muted" style="margin:0;flex:1 1 auto">
				On your calendar from {startsOn}. <a href="/calendar">Open the calendar</a>.
			</p>
		{:else}
			<button style="flex:0 0 auto" onclick={addToCalendar} disabled={saving}>
				{saving ? 'Adding' : 'Add to my calendar'}
			</button>
			<p class="muted" style="margin:0;font-size:0.85rem;flex:1 1 200px">
				Schedules every session from {startsOn}.
			</p>
		{/if}
	</div>

	{#if saveError}
		<div class="notice error" style="margin-top:0.7rem">{saveError}</div>
	{/if}

	{#if plan.restrictions?.length}
		<div class="notice form-width" style="margin-top:0.9rem">
			<strong>Adjusted for:</strong>
			<ul style="margin:0.4rem 0 0;padding-left:1.1rem">
				{#each plan.restrictions as restriction}
					<li>{restriction}</li>
				{/each}
			</ul>
		</div>
	{/if}

	{#if warnings.length}
		<div class="notice error form-width" style="margin-top:0.7rem">
			<strong>Trimmed before you saw it:</strong>
			<ul style="margin:0.4rem 0 0;padding-left:1.1rem">
				{#each warnings as warning}
					<li>{warning}</li>
				{/each}
			</ul>
		</div>
	{/if}

	{#if plan.phases?.length || plan.progression_rules?.length || plan.test}
		<div class="grid" style="margin-top:1rem">
			{#each plan.phases ?? [] as phase (phase.weeks)}
				<div class="panel">
					<p class="eyebrow">Weeks {phase.weeks}</p>
					<p style="font-weight:600;margin:0.2rem 0 0.2rem">{phase.name}</p>
					<p class="muted" style="font-size:0.85rem;margin:0">{phase.aim}</p>
				</div>
			{/each}
		</div>

		{#if plan.progression_rules?.length}
			<p class="eyebrow" style="margin-top:1.2rem">How the load moves</p>
			<ul class="muted" style="margin:0.3rem 0 0;padding-left:1.1rem;font-size:0.9rem">
				{#each plan.progression_rules as rule}
					<li>{rule}</li>
				{/each}
			</ul>
		{/if}

		{#if plan.test}
			<p class="eyebrow" style="margin-top:1.2rem">The test</p>
			<p style="margin:0.2rem 0 0;font-size:0.92rem">{plan.test}</p>
		{/if}
	{/if}

	<div class="bar"></div>
	<p class="eyebrow">The plan, week by week</p>
	<p class="muted" style="font-size:0.85rem;margin:0.3rem 0 0">
		Dates come from the start date above. Open a session to see the work in it.
	</p>

	<div class="cal">
		<div class="cal-dow"></div>
		{#each DAY_SHORT as name (name)}
			<div class="cal-dow">{name}</div>
		{/each}

		{#each byWeek as week (week.week)}
			<div class="cal-wk">
				W{week.week}
				<span class="count">{week.sessions}×</span>
			</div>

			{#each week.slots as slot (slot.day)}
				<div class="cal-cell" class:rest={!slot.items.length}>
					<span class="cal-date">
						<span class="dow-inline">{slot.short}</span>
						{dayLabel(slot.date)}
					</span>

					{#each slot.items as session, i (i)}
						<button
							class="cal-item"
							class:hard={session.load === 'hard'}
							class:moderate={session.load === 'moderate'}
							class:light={session.load === 'easy' || session.load === 'deload'}
							class:on={openKey === keyOf(week.week, slot.day, i)}
							onclick={() => {
								const key = keyOf(week.week, slot.day, i);
								openKey = openKey === key ? '' : key;
							}}
						>
							<span class="title">{session.title}</span>
							<!-- How hard the session is, is the colour of the bar. -->
							<span class="meta">{sessionShape(session)}</span>
						</button>
					{/each}
				</div>
			{/each}

			{#if openSession && openSession.week === week.week}
					<div class="cal-detail">
						<div class="cal-detail-head">
							<p class="eyebrow" style="margin:0">
								Week {week.week}{` · ${formatDate(isoDate(openSession.slot.date))}`}
							</p>
							<strong style="flex:1 1 auto">{openSession.session.title}</strong>
							{#if openSession.session.load}
								<span
									class="chip"
									class:hard={openSession.session.load === 'hard'}
									class:deload={openSession.session.load === 'deload'}
								>{openSession.session.load}</span
								>
							{/if}
							<button class="link" onclick={() => (openKey = '')}>Close</button>
						</div>
						<p class="muted" style="margin:0 0 0.6rem;font-size:0.88rem">
							{openSession.session.focus}{#if openSession.session.duration_minutes}{` · ${openSession.session.duration_minutes} min`}{/if}
						</p>
						<SessionDetail session={openSession.session} />
				</div>
			{/if}
		{/each}
	</div>

	{#if plan.research}
		<div class="bar"></div>
		<h2>What the coach read</h2>
		<p class="muted" style="font-size:0.88rem;margin:0.3rem 0 0.8rem">
			{plan.research.searches_used} searches{#if plan.research.cached}, reused from earlier
				research{/if}. The plan was written against these.
		</p>
		{#if plan.research.summary}
			<p class="prose panel">{plan.research.summary}</p>
		{/if}
		{#if plan.research.progression?.length}
			<p class="eyebrow" style="margin-top:1rem">The ladder</p>
			<table style="margin-top:0.4rem">
				<thead>
					<tr><th>Rung</th><th>Exercises</th><th>Hold to</th></tr>
				</thead>
				<tbody>
					{#each plan.research.progression as stage, i (i)}
						<tr>
							<td>{stage.stage}</td>
							<td class="mono muted">{(stage.exercise_slugs ?? []).join(', ')}</td>
							<td>{stage.standard}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
		{#if plan.research.sources?.length}
			<p class="eyebrow" style="margin-top:1rem">Sources</p>
			<ul style="margin:0.3rem 0 0;padding-left:1.1rem;font-size:0.88rem">
				{#each plan.research.sources as source (source.url)}
					<li>
						<a href={source.url} target="_blank" rel="noreferrer noopener"
							>{source.title || source.url}</a
						>
					</li>
				{/each}
			</ul>
		{/if}
	{/if}
{/if}
