<script>
	import { api } from '$lib/api.js';
	import { session } from '$lib/session.svelte.js';
	import SessionDetail from '$lib/SessionDetail.svelte';
	import { DAY_SHORT, sessionShape, isoDate, mondayOf, addDays, formatDate } from '$lib/week.js';

	// Five weeks is a month view: the block a training cycle is read in, and
	// what fits on a screen without scrolling the header away.
	const WEEKS_SHOWN = 5;

	let anchor = $state(mondayOf(new Date()));
	let calendar = $state(null);
	let plans = $state([]);
	let error = $state('');
	let busyId = $state('');
	let openId = $state('');

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

	// The API answers with a flat list of dated sessions; the grid wants them
	// dealt out into the day each one falls on.
	let weeks = $derived.by(() => {
		const sessions = calendar?.sessions ?? [];
		const events = calendar?.events ?? [];
		return Array.from({ length: WEEKS_SHOWN }, (_, w) => {
			const monday = addDays(from, w * 7);
			const days = Array.from({ length: 7 }, (_, d) => {
				const date = addDays(monday, d);
				const iso = isoDate(date);
				return {
					iso,
					date,
					short: DAY_SHORT[d],
					entries: sessions.filter((s) => s.scheduled_on === iso),
					events: events.filter((e) => e.starts_on === iso)
				};
			});
			const entries = days.flatMap((day) => day.entries);
			return {
				monday,
				days,
				sessions: entries.length,
				completed: entries.filter((e) => e.completed_at).length
			};
		});
	});

	let scheduled = $derived((calendar?.sessions ?? []).length);
	let open = $derived(
		(calendar?.sessions ?? []).find((s) => s.id === openId) ?? null
	);

	// The month a cell belongs to is only worth saying on the first of it.
	function dayLabel(date) {
		const day = date.getDate();
		return day === 1 ? formatDate(isoDate(date)) : String(day);
	}

	function shift(weeks) {
		anchor = addDays(anchor, weeks * 7);
		openId = '';
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
			openId = '';
		} catch (e) {
			error = e.message;
		} finally {
			busyId = '';
		}
	}
</script>

<p class="eyebrow" style="margin-top:2rem">Schedule</p>
<h1>Your calendar</h1>
<p class="muted column" style="margin-top:0.6rem">
	Every session a plan put on the calendar, on the day it falls. Open one to see the work; tick it
	off when it is done.
</p>

{#if error}
	<div class="notice error" style="margin-top:1rem">{error}</div>
{/if}

<div class="bar"></div>

<div class="row" style="align-items:center">
	<button class="ghost" style="flex:0 0 auto" onclick={() => shift(-WEEKS_SHOWN)}>← Earlier</button>
	<button class="ghost" style="flex:0 0 auto" onclick={() => (anchor = mondayOf(new Date()))}>
		Today
	</button>
	<button class="ghost" style="flex:0 0 auto" onclick={() => shift(WEEKS_SHOWN)}>Later →</button>
	<p class="mono muted" style="margin:0;flex:1 1 auto;text-align:right;font-size:0.85rem">
		{formatDate(isoDate(from))} – {formatDate(isoDate(to))}{` · ${scheduled} sessions`}
	</p>
</div>

<div class="cal">
	<div class="cal-dow"></div>
	{#each DAY_SHORT as name (name)}
		<div class="cal-dow">{name}</div>
	{/each}

	{#each weeks as week (isoDate(week.monday))}
		<div class="cal-wk">
			{formatDate(isoDate(week.monday))}
			{#if week.sessions}
				<span class="count">{week.completed}/{week.sessions}</span>
			{/if}
		</div>

		{#each week.days as day (day.iso)}
			<div
				class="cal-cell"
				class:rest={!day.entries.length && !day.events.length}
				class:today={day.iso === today}
			>
				<span class="cal-date">
					<span class="dow-inline">{day.short}</span>
					{dayLabel(day.date)}
				</span>

				{#each day.events as event (event.id)}
					<span class="cal-item event" style="cursor:default">
						<span class="title">{event.name}</span>
						<span class="meta">{event.city || event.country || 'Competition'}</span>
					</span>
				{/each}

				{#each day.entries as entry (entry.id)}
					<button
						class="cal-item"
						class:on={openId === entry.id}
						class:done={entry.completed_at}
						onclick={() => (openId = openId === entry.id ? '' : entry.id)}
					>
						<span class="title">{entry.title}</span>
						<span class="meta">{sessionShape(entry.body)}</span>
					</button>
				{/each}
			</div>
		{/each}

		{#if open && week.days.some((day) => day.iso === open.scheduled_on)}
				<div class="cal-detail">
					<div class="cal-detail-head">
					<p class="eyebrow" style="margin:0">{formatDate(open.scheduled_on)}</p>
					<strong style="flex:1 1 auto">{open.title}</strong>
					<button
						class="ghost"
						style="padding:0.35rem 0.7rem;font-size:0.8rem"
						onclick={() => toggle(open)}
						disabled={busyId === open.id}
					>
						{open.completed_at ? 'Done — undo' : 'Mark done'}
					</button>
					<button class="link" onclick={() => (openId = '')}>Close</button>
				</div>
				<p class="muted" style="margin:0 0 0.6rem;font-size:0.88rem">{open.focus}</p>
				<SessionDetail session={open.body} />
			</div>
		{/if}
	{/each}
</div>

{#if calendar && scheduled === 0}
	<div class="empty" style="margin-top:1rem">
		Nothing scheduled in these five weeks. <a href="/plan">Build a plan</a> and add it to your
		calendar.
	</div>
{/if}

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
						<button class="link" onclick={() => removePlan(plan)} disabled={busyId === plan.id}>
							Remove
						</button>
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
