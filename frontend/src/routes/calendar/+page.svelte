<script>
	import { api } from '$lib/api.js';
	import { page } from '$app/state';
	import { session } from '$lib/session.svelte.js';
	import SessionEditor from '$lib/SessionEditor.svelte';
	import {
		DAY_SHORT,
		sessionShape,
		blankSession,
		cleanBody,
		editableBody,
		isoDate,
		mondayOf,
		addDays,
		formatDate
	} from '$lib/week.js';

	// Five weeks is a month view: the block a training cycle is read in, and
	// what fits on a screen without scrolling the header away.
	const WEEKS_SHOWN = 5;

	let anchor = $state(mondayOf(new Date()));
	let calendar = $state(null);
	let plans = $state([]);
	let exercises = $state([]);
	let error = $state('');
	let busyId = $state('');
	// The session being written, if any: a new one on a day, or an existing
	// one opened for correction.
	let editor = $state(null);

	const today = isoDate(new Date());

	let from = $derived(anchor);
	let to = $derived(addDays(anchor, WEEKS_SHOWN * 7 - 1));

	$effect(() => {
		if (!session.user || exercises.length) return;
		api.get('/exercises')
			.then((library) => (exercises = library))
			.catch((e) => (error = e.message));
	});

	// reloads is bumped to ask for the calendar again without moving the window.
	let reloads = $state(0);

	$effect(() => {
		if (!session.user) return;
		reloads;
		const query = `?from=${isoDate(from)}&to=${isoDate(to)}`;
		Promise.all([api.get(`/calendar${query}`), api.get('/plans')])
			.then(([cal, saved]) => {
				calendar = cal;
				plans = saved;
			})
			.catch((e) => (error = e.message));
	});

	// A session is performed in its own tab, and marked done there. Coming back
	// to this one should not show a week that is out of date by an hour.
	$effect(() => {
		const refresh = () => (reloads += 1);
		window.addEventListener('focus', refresh);
		return () => window.removeEventListener('focus', refresh);
	});

	// The session page sends corrections back here: ?edit=<id> opens that
	// session in the editor, and ?on=<date> makes sure the week holding it is
	// the week being shown, since the editor works from the loaded calendar.
	let editRequest = $state('');
	$effect(() => {
		const wanted = page.url.searchParams.get('edit') ?? '';
		const on = page.url.searchParams.get('on') ?? '';
		if (!wanted || wanted === editRequest) return;
		editRequest = wanted;
		if (on) anchor = mondayOf(new Date(`${on}T00:00:00`));
	});

	$effect(() => {
		if (!editRequest) return;
		const found = (calendar?.sessions ?? []).find((s) => s.id === editRequest);
		if (found) {
			startEdit(found);
			editRequest = '';
		}
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
	// The month a cell belongs to is only worth saying on the first of it.
	function dayLabel(date) {
		const day = date.getDate();
		return day === 1 ? formatDate(isoDate(date)) : String(day);
	}

	function shift(weeks) {
		anchor = addDays(anchor, weeks * 7);
	}

	// ---------- writing sessions straight onto a day ----------

	function startNew(iso) {
		editor = { id: '', scheduled_on: iso, body: blankSession('') };
	}

	function startEdit(entry) {
		editor = { id: entry.id, scheduled_on: entry.scheduled_on, body: editableBody(entry.body) };
	}

	async function saveSession() {
		error = '';
		busyId = 'editor';
		try {
			const payload = { scheduled_on: editor.scheduled_on, body: cleanBody(editor.body) };
			const sessions = calendar?.sessions ?? [];
			if (editor.id) {
				const updated = await api.patch(`/sessions/${editor.id}`, payload);
				calendar = { ...calendar, sessions: sessions.map((s) => (s.id === updated.id ? updated : s)) };
			} else {
				const created = await api.post('/sessions', payload);
				calendar = { ...calendar, sessions: [...sessions, created] };
			}
			editor = null;
		} catch (e) {
			error = e.message;
		} finally {
			busyId = '';
		}
	}

	async function removeSession(entry) {
		if (!confirm(`Remove "${entry.title}" from ${formatDate(entry.scheduled_on)}?`)) return;
		error = '';
		busyId = entry.id;
		try {
			await api.del(`/sessions/${entry.id}`);
			calendar = {
				...calendar,
				sessions: (calendar?.sessions ?? []).filter((s) => s.id !== entry.id)
			};
			if (editor?.id === entry.id) editor = null;
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
<p class="muted column" style="margin-top:0.6rem">
	Every session on the day it falls, wherever it came from: a plan, your repeating routine, or typed
	straight onto a day here. Opening one gives you the session in its own tab, in the order it is
	performed — joints, warm-up, mobility, the specific warm-up, the training, the cool-down.
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
	<p class="mono note" style="margin:0;flex:1 1 auto;text-align:right">
		{formatDate(isoDate(from))} – {formatDate(isoDate(to))}{` · ${scheduled} sessions`}
	</p>
</div>

{#if editor}
	<div class="panel" style="margin-top:0.9rem">
		<div class="row" style="align-items:flex-end">
			<div style="flex:1 1 auto">
				<p class="eyebrow" style="margin:0 0 0.2rem">
					{editor.id ? 'Edit session' : 'New session'}
				</p>
				<strong class="item-title">{formatDate(editor.scheduled_on)}</strong>
			</div>
			<div style="flex:0 1 170px">
				<label for="editor-date">Day</label>
				<input id="editor-date" type="date" bind:value={editor.scheduled_on} />
			</div>
		</div>

		<div style="margin-top:0.8rem">
			<SessionEditor body={editor.body} {exercises} />
		</div>

		<div class="row" style="margin-top:1.1rem">
			<button
				style="flex:0 0 auto"
				onclick={saveSession}
				disabled={busyId === 'editor' || !editor.body.title.trim()}
			>
				{busyId === 'editor' ? 'Saving…' : editor.id ? 'Save changes' : 'Add to calendar'}
			</button>
			<button class="ghost" style="flex:0 0 auto" onclick={() => (editor = null)}>Cancel</button>
			{#if editor.id}
				{@const entry = (calendar?.sessions ?? []).find((s) => s.id === editor.id)}
				{#if entry}
					<button
						class="link"
						style="flex:0 0 auto"
						onclick={() => removeSession(entry)}
						disabled={busyId === entry.id}
					>
						Remove
					</button>
				{/if}
			{/if}
			<p class="note" style="flex:1 1 auto;margin:0;text-align:right">
				Training the same week every week? <a href="/routine">Save it as a routine</a> instead.
			</p>
		</div>
	</div>
{/if}

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
					<a
						class="cal-item"
						class:done={entry.completed_at}
						href={`/session/${entry.id}`}
						target="_blank"
						rel="noopener"
						title={`Open ${entry.title} in a new tab`}
					>
						<span class="title">{entry.title}</span>
						<span class="meta">{sessionShape(entry.body)}</span>
					</a>
				{/each}

				<button
					class="cal-add"
					title={`Add a session on ${formatDate(day.iso)}`}
					aria-label={`Add a session on ${formatDate(day.iso)}`}
					onclick={() => startNew(day.iso)}
				>
					+
				</button>
			</div>
		{/each}

	{/each}
</div>

{#if calendar && scheduled === 0}
	<div class="empty" style="margin-top:1rem">
		Nothing scheduled in these five weeks. <a href="/routine">Save the week you already train</a>,
		<a href="/plan">build a plan</a>, or press + on a day to write one session.
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
<p class="lede" style="margin:0.3rem 0 0.9rem">
	Every scheduled session as an .ics file, for the calendar app you already use.
</p>
<a class="mono" href="/api/v1/calendar.ics" download="training.ics">Download training.ics</a>

<style>
	/* One press on a day is the whole path to writing a session there, so the
	   button sits in the cell rather than behind a menu — quiet until the day
	   is hovered or it is focused for the keyboard. */
	.cal-add {
		background: none;
		border: 1px dashed var(--line);
		color: var(--muted);
		font-family: var(--mono);
		font-size: 0.9rem;
		line-height: 1;
		padding: 0.15rem 0;
		margin-top: auto;
		border-radius: var(--radius);
		opacity: 0;
		transition: opacity 0.12s ease;
	}
	:global(.cal-cell):hover .cal-add,
	.cal-add:focus-visible {
		opacity: 1;
	}
	.cal-add:hover {
		border-color: var(--signal);
		color: var(--signal-text);
		filter: none;
	}
	@media (hover: none) {
		/* Nothing hovers on a phone, so the button is simply always there. */
		.cal-add {
			opacity: 0.6;
		}
	}
</style>
