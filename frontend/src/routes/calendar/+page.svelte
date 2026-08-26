<script>
	import { api } from '$lib/api.js';
	import { session } from '$lib/session.svelte.js';
	import SessionDetail from '$lib/SessionDetail.svelte';
	import { weekSlots, sessionShape, isoDate, mondayOf, addDays, isoDay, formatDate } from '$lib/week.js';

	// How much of the calendar is on screen at once. Four weeks is the unit a
	// training block is read in, and it fits the four-row grid four times over.
	const WEEKS_SHOWN = 4;

	let anchor = $state(mondayOf(new Date()));
	let calendar = $state(null);
	let plans = $state([]);
	let error = $state('');
	let busyId = $state('');

	const today = isoDate(new Date());

	let from = $derived(anchor);
	let to = $derived(addDays(anchor, WEEKS_SHOWN * 7 - 1));

	$effect(() => {
		if (!session.user) return;
		const query = `?from=${isoDate(from)}&to=${isoDate(to)}`;
		Promise.all([api.get(`/calendar${query}`), api.get('/plans')])
			.then(([cal, saved]) => {
				calendar = cal;
				plans = saved;
			})
			.catch((e) => (error = e.message));
	});

	// The API answers with a flat list of dated sessions. The calendar shows
	// weeks, so they are dealt back out into the squares they belong in.
	let weeks = $derived.by(() => {
		const sessions = calendar?.sessions ?? [];
		const events = calendar?.events ?? [];
		return Array.from({ length: WEEKS_SHOWN }, (_, i) => {
			const monday = addDays(from, i * 7);
			const sunday = addDays(monday, 6);
			const inWeek = sessions.filter((s) => s.scheduled_on >= isoDate(monday) && s.scheduled_on <= isoDate(sunday));
			const eventsInWeek = events.filter((e) => e.starts_on >= isoDate(monday) && e.starts_on <= isoDate(sunday));
			const slots = weekSlots(inWeek, (s) => isoDay(new Date(`${s.scheduled_on}T00:00:00`)));
			for (const event of eventsInWeek) {
				const day = isoDay(new Date(`${event.starts_on}T00:00:00`));
				slots[day - 1].event = event;
			}
			return {
				monday,
				slots: slots.map((slot, day) => ({ ...slot, date: addDays(monday, day) })),
				sessions: inWeek.length,
				completed: inWeek.filter((s) => s.completed_at).length,
				sets: inWeek.reduce(
					(total, s) => total + (s.body?.blocks ?? []).reduce((n, b) => n + (Number(b.sets) || 0), 0),
					0
				)
			};
		});
	});

	let scheduled = $derived((calendar?.sessions ?? []).length);

	function shift(weeks) {
		anchor = addDays(anchor, weeks * 7);
	}

	async function toggle(entry) {
		busyId = entry.id;
		error = '';
		try {
			if (entry.completed_at) {
				await api.del(`/sessions/${entry.id}/complete`);
				entry.completed_at = null;
			} else {
				await api.post(`/sessions/${entry.id}/complete`, {});
				entry.completed_at = new Date().toISOString();
			}
		} catch (e) {
			error = e.message;
		} finally {
			busyId = '';
		}
	}

	async function removePlan(plan) {
		if (!confirm(`Remove "${plan.title}" and its ${plan.sessions} scheduled sessions?`)) return;
		busyId = plan.id;
		error = '';
		try {
			await api.del(`/plans/${plan.id}`);
			plans = plans.filter((p) => p.id !== plan.id);
			calendar = {
				...calendar,
				sessions: (calendar?.sessions ?? []).filter((s) => s.plan_id !== plan.id)
			};
		} catch (e) {
			error = e.message;
		} finally {
			busyId = '';
		}
	}
</script>

<p class="eyebrow" style="margin-top:2rem">Schedule</p>
<h1>Your calendar</h1>
<p class="muted" style="margin-top:0.6rem;max-width:56ch">
	Every session a plan put on the calendar, in the week it falls in. Tick one off when it is done,
	and the plan shows how far through it you are.
</p>

{#if error}
	<div class="notice error" style="margin-top:1rem">{error}</div>
{/if}

<div class="bar"></div>

<div class="row" style="align-items:center">
	<button class="ghost" style="flex:0 0 auto" onclick={() => shift(-WEEKS_SHOWN)}>← Earlier</button>
	<button class="ghost" style="flex:0 0 auto" onclick={() => (anchor = mondayOf(new Date()))}>
		This week
	</button>
	<button class="ghost" style="flex:0 0 auto" onclick={() => shift(WEEKS_SHOWN)}>Later →</button>
	<p class="mono muted" style="margin:0;flex:1 1 auto;text-align:right;font-size:0.85rem">
		{formatDate(isoDate(from))} – {formatDate(isoDate(to))} · {scheduled} sessions
	</p>
</div>

{#if calendar && scheduled === 0}
	<div class="empty" style="margin-top:1rem">
		Nothing scheduled in these four weeks. <a href="/plan">Build a plan</a> and add it to your
		calendar.
	</div>
{/if}

{#each weeks as week (isoDate(week.monday))}
	<div class="bar"></div>
	<p class="eyebrow">
		Week of {formatDate(isoDate(week.monday))}
	</p>

	<!-- Monday and Tuesday, then a break, and so on: four rows of two. -->
	<div class="week-grid">
		{#each week.slots as slot (slot.day)}
			<div
				class="panel day"
				class:rest={!slot.items.length && !slot.event}
				class:today={isoDate(slot.date) === today}
				class:done={slot.items.length > 0 && slot.items.every((s) => s.completed_at)}
			>
				<div class="day-head">
					<p class="eyebrow" style="margin:0">{slot.short} {formatDate(isoDate(slot.date))}</p>
					{#if slot.items.length}
						<span class="chip">{slot.items.filter((s) => s.completed_at).length}/{slot.items.length}</span
						>
					{/if}
				</div>

				{#if slot.event}
					<p class="chip hard" style="align-self:flex-start;margin:0.3rem 0 0.1rem">Competition</p>
					<p style="font-weight:600;margin:0 0 0.1rem">{slot.event.name}</p>
					<p class="muted" style="font-size:0.82rem;margin:0">
						{slot.event.city}{#if slot.event.country}{`, ${slot.event.country}`}{/if}
					</p>
				{/if}

				{#each slot.items as entry (entry.id)}
					<div style="margin-top:0.2rem">
						<p style="font-weight:600;margin:0 0 0.1rem">{entry.title}</p>
						<p class="muted" style="font-size:0.85rem;margin:0">{entry.focus}</p>
						<p class="mono muted" style="font-size:0.74rem;margin:0.3rem 0 0">
							{sessionShape(entry.body)}
						</p>

						<details style="margin-top:0.45rem">
							<summary class="eyebrow" style="cursor:pointer">The session</summary>
							<div style="margin-top:0.5rem">
								<SessionDetail session={entry.body} />
							</div>
						</details>

						<button
							class="ghost"
							style="margin-top:0.55rem;padding:0.35rem 0.7rem;font-size:0.8rem"
							onclick={() => toggle(entry)}
							disabled={busyId === entry.id}
						>
							{entry.completed_at ? 'Done — undo' : 'Mark done'}
						</button>
					</div>
				{/each}

				{#if !slot.items.length && !slot.event}
					<p class="muted" style="margin:0.1rem 0 0;font-size:0.85rem">Rest</p>
				{/if}
			</div>
		{/each}

		<!-- The eighth square: what the week asks of you, and how much of it is done. -->
		<div class="panel day summary">
			<p class="eyebrow" style="margin:0">The week</p>
			<p class="stat" style="margin:0.1rem 0 0;font-size:1.15rem">
				{week.completed}/{week.sessions} done
			</p>
			<div class="rig">
				{#each Array(Math.max(week.sessions, 1)) as _, i}
					<span class:on={i < week.completed}></span>
				{/each}
			</div>
			<p class="muted" style="font-size:0.8rem;margin:0.5rem 0 0">
				{week.sets} sets scheduled
			</p>
		</div>
	</div>
{/each}

<div class="bar"></div>
<h2>Plans on your calendar</h2>

{#if plans.length === 0}
	<div class="empty" style="margin-top:0.8rem">
		No plans saved yet. <a href="/plan">Build one</a>.
	</div>
{:else}
	<table style="margin-top:0.8rem">
		<thead>
			<tr><th>Plan</th><th>Runs</th><th>Progress</th><th></th></tr>
		</thead>
		<tbody>
			{#each plans as plan (plan.id)}
				<tr>
					<td>
						{plan.title}
						{#if plan.goal}<span class="muted"> — {plan.goal}</span>{/if}
					</td>
					<td class="mono muted">
						{formatDate(plan.starts_on)}{#if plan.ends_on}{` – ${formatDate(plan.ends_on)}`}{/if}
						{` · ${plan.weeks} weeks`}
					</td>
					<td class="mono">{plan.completed}/{plan.sessions}</td>
					<td style="text-align:right">
						<button
							class="link"
							onclick={() => removePlan(plan)}
							disabled={busyId === plan.id}>Remove</button
						>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
{/if}

<div class="bar"></div>
<h2>Take it with you</h2>
<p class="muted" style="font-size:0.88rem;margin:0.3rem 0 0.9rem">
	Every scheduled session as an .ics file, for the calendar app you already use.
</p>
<a class="mono" href="/api/v1/calendar.ics" download="training.ics">Download training.ics</a>
