<script>
	import { api } from '$lib/api.js';
	import { session } from '$lib/session.svelte.js';
	import SessionEditor from '$lib/SessionEditor.svelte';
	import { DAY_SHORT, blankSession, cleanBody, editableBody, isoDate, mondayOf, addDays, formatDate } from '$lib/week.js';

	const DAY_NAMES = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];

	let exercises = $state([]);
	let routines = $state([]);
	let draft = $state(null);
	let openIndex = $state(-1);
	let error = $state('');
	let notice = $state('');
	let busy = $state(false);
	let applyWeek = $state(isoDate(mondayOf(new Date())));

	$effect(() => {
		if (!session.user) return;
		Promise.all([api.get('/exercises'), api.get('/routines')])
			.then(([library, saved]) => {
				exercises = library;
				routines = saved;
			})
			.catch((e) => (error = e.message));
	});

	function startNew() {
		draft = {
			id: null,
			title: '',
			notes: '',
			repeat: 'weekly',
			week_of: isoDate(mondayOf(new Date())),
			sessions: []
		};
		openIndex = -1;
		notice = '';
	}

	function startEdit(routine) {
		draft = {
			id: routine.id,
			title: routine.title,
			notes: routine.notes,
			repeat: routine.active ? 'weekly' : 'once',
			week_of: routine.starts_on,
			sessions: routine.days.map((day) => ({
				day_of_week: day.day_of_week,
				body: editableBody(day.body)
			}))
		};
		openIndex = -1;
		notice = '';
	}

	function addSession(dayOfWeek) {
		draft.sessions = [
			...draft.sessions,
			{ day_of_week: dayOfWeek, body: blankSession(`${DAY_SHORT[dayOfWeek - 1]} session`) }
		];
		openIndex = draft.sessions.length - 1;
	}

	function removeSession(index) {
		draft.sessions = draft.sessions.filter((_, i) => i !== index);
		openIndex = -1;
	}

	// The sessions are held in one flat list so a day can be changed without
	// moving anything; the week is grouped for display only.
	let week = $derived(
		DAY_NAMES.map((name, i) => ({
			day: i + 1,
			name,
			short: DAY_SHORT[i],
			sessions: draft ? draft.sessions.map((s, index) => ({ ...s, index })).filter((s) => s.day_of_week === i + 1) : []
		}))
	);

	let sessionCount = $derived(draft?.sessions.length ?? 0);

	async function save() {
		error = '';
		notice = '';
		busy = true;
		try {
			const days = draft.sessions.map((s) => ({
				day_of_week: Number(s.day_of_week),
				body: cleanBody(s.body)
			}));
			if (draft.id) {
				const updated = await api.patch(`/routines/${draft.id}`, {
					title: draft.title.trim(),
					notes: draft.notes.trim(),
					days
				});
				routines = routines.map((r) => (r.id === updated.id ? updated : r));
				notice = `"${updated.title}" updated. The weeks ahead have been rewritten.`;
			} else {
				const created = await api.post('/routines', {
					title: draft.title.trim(),
					notes: draft.notes.trim(),
					repeat: draft.repeat,
					week_of: draft.week_of,
					days
				});
				routines = [created, ...routines];
				notice =
					draft.repeat === 'weekly'
						? `"${created.title}" is on your calendar every week from ${formatDate(created.starts_on)}.`
						: `"${created.title}" has been put on the week of ${formatDate(created.starts_on)}.`;
			}
			draft = null;
		} catch (e) {
			error = e.message;
		} finally {
			busy = false;
		}
	}

	async function toggleActive(routine) {
		error = '';
		notice = '';
		busy = true;
		try {
			const updated = await api.patch(`/routines/${routine.id}`, { active: !routine.active });
			routines = routines.map((r) => (r.id === updated.id ? updated : r));
			notice = updated.active
				? `"${updated.title}" is repeating again.`
				: `"${updated.title}" has stopped. The sessions it had ahead of today are off the calendar; what is done stays.`;
		} catch (e) {
			error = e.message;
		} finally {
			busy = false;
		}
	}

	async function applyToWeek(routine) {
		error = '';
		notice = '';
		busy = true;
		try {
			const result = await api.post(`/routines/${routine.id}/apply`, { week_of: applyWeek });
			const from = formatDate(result.week_of);
			notice =
				result.added > 0
					? `${result.added} session${result.added === 1 ? '' : 's'} added to the week of ${from}.`
					: `That week already has every session from "${routine.title}".`;
			routines = await api.get('/routines');
		} catch (e) {
			error = e.message;
		} finally {
			busy = false;
		}
	}

	async function remove(routine) {
		if (
			!confirm(
				`Remove "${routine.title}"? Its sessions from today on come off the calendar. Anything already done stays.`
			)
		)
			return;
		error = '';
		notice = '';
		busy = true;
		try {
			await api.del(`/routines/${routine.id}`);
			routines = routines.filter((r) => r.id !== routine.id);
			if (draft?.id === routine.id) draft = null;
		} catch (e) {
			error = e.message;
		} finally {
			busy = false;
		}
	}

	function summarise(routine) {
		if (!routine.days.length) return 'No sessions yet';
		const days = [...new Set(routine.days.map((d) => DAY_SHORT[d.day_of_week - 1]))];
		return `${routine.days.length} session${routine.days.length === 1 ? '' : 's'} · ${days.join(', ')}`;
	}

	let applyWeekEnd = $derived(formatDate(isoDate(addDays(new Date(`${applyWeek}T00:00:00`), 6))));
</script>

<p class="eyebrow" style="margin-top:2rem">Your week</p>
<h1>Training routine</h1>
<p class="muted column" style="margin-top:0.6rem">
	What you already train, written once. Keep it repeating and every week ahead fills itself, or drop
	it onto a single week and leave the rest alone. Sessions land on the calendar like any other, and
	are ticked off the same way.
</p>

{#if error}
	<div class="notice error" style="margin-top:1rem">{error}</div>
{/if}
{#if notice}
	<div class="notice" style="margin-top:1rem">{notice}</div>
{/if}

<div class="bar"></div>

{#if !draft}
	<div class="row" style="align-items:center">
		<h2 style="margin:0;flex:1 1 auto">Saved routines</h2>
		<button style="flex:0 0 auto" onclick={startNew}>New routine</button>
	</div>

	{#if routines.length === 0}
		<div class="empty" style="margin-top:0.9rem">
			Nothing saved yet. Write the week you actually train — the days you go, and what you do on
			them — and it goes on the calendar from there.
		</div>
	{:else}
		<div style="display:grid;gap:0.8rem;margin-top:0.9rem">
			{#each routines as routine (routine.id)}
				<div class="panel">
					<div class="row" style="align-items:baseline">
						<strong style="flex:1 1 auto">
							{routine.title}
							<span class="tag" class:on={routine.active}>
								{routine.active ? 'Every week' : 'Paused'}
							</span>
						</strong>
						<span class="mono muted" style="flex:0 0 auto;font-size:0.82rem">
							{summarise(routine)}{routine.upcoming ? ` · ${routine.upcoming} ahead` : ''}
						</span>
					</div>

					{#if routine.notes}
						<p class="muted" style="margin:0.4rem 0 0;font-size:0.88rem">{routine.notes}</p>
					{/if}

					<div class="row" style="margin-top:0.7rem;align-items:flex-end">
						<div style="flex:0 1 170px">
							<label for={`week-${routine.id}`}>Put on the week of</label>
							<input id={`week-${routine.id}`} type="date" bind:value={applyWeek} />
						</div>
						<button
							class="ghost"
							style="flex:0 0 auto"
							onclick={() => applyToWeek(routine)}
							disabled={busy}
						>
							Add to that week only
						</button>
						<span class="spacer"></span>
						<button class="ghost" style="flex:0 0 auto" onclick={() => startEdit(routine)}>
							Edit
						</button>
						<button
							class="ghost"
							style="flex:0 0 auto"
							onclick={() => toggleActive(routine)}
							disabled={busy}
						>
							{routine.active ? 'Stop repeating' : 'Repeat every week'}
						</button>
						<button class="link" style="flex:0 0 auto" onclick={() => remove(routine)} disabled={busy}>
							Remove
						</button>
					</div>
					<p class="muted" style="margin:0.5rem 0 0;font-size:0.78rem">
						That week runs to {applyWeekEnd}. Days already holding this routine are left as they are.
					</p>
				</div>
			{/each}
		</div>
	{/if}
{:else}
	<h2 style="margin:0">{draft.id ? 'Edit routine' : 'New routine'}</h2>

	<div class="row" style="margin-top:0.9rem">
		<div style="flex:2 1 240px">
			<label for="routine-title">Name</label>
			<input id="routine-title" bind:value={draft.title} placeholder="Current week" maxlength="120" />
		</div>
		<div style="flex:3 1 280px">
			<label for="routine-notes">Notes</label>
			<input
				id="routine-notes"
				bind:value={draft.notes}
				placeholder="what this block is for"
				maxlength="500"
			/>
		</div>
	</div>

	{#if !draft.id}
		<div class="row" style="margin-top:0.6rem">
			<div style="flex:1 1 220px">
				<label for="routine-repeat">How often</label>
				<select id="routine-repeat" bind:value={draft.repeat}>
					<option value="weekly">Every week, from that week on</option>
					<option value="once">Just that one week</option>
				</select>
			</div>
			<div style="flex:0 1 180px">
				<label for="routine-week">Starting the week of</label>
				<input id="routine-week" type="date" bind:value={draft.week_of} />
			</div>
		</div>
		<p class="muted" style="margin:0.4rem 0 0;font-size:0.82rem">
			{draft.repeat === 'weekly'
				? 'Every week keeps filling ahead until you stop it. Days you clear stay cleared.'
				: 'One week only. The routine stays saved, so you can put it on another week whenever you want.'}
		</p>
	{/if}

	<div class="bar"></div>

	<div style="display:grid;gap:0.5rem">
		{#each week as day (day.day)}
			<div class="day" class:rest={day.sessions.length === 0}>
				<div class="row" style="align-items:baseline">
					<strong style="flex:0 0 110px">{day.name}</strong>
					<span class="muted" style="flex:1 1 auto;font-size:0.85rem">
						{day.sessions.length === 0 ? 'Rest' : day.sessions.map((s) => s.body.title || 'Untitled').join(' · ')}
					</span>
					<button class="ghost small" style="flex:0 0 auto" onclick={() => addSession(day.day)}>
						Add session
					</button>
				</div>

				{#each day.sessions as entry (entry.index)}
					<div class="session">
						<div class="row" style="align-items:center">
							<button
								class="link"
								style="flex:1 1 auto;text-align:left"
								onclick={() => (openIndex = openIndex === entry.index ? -1 : entry.index)}
							>
								{openIndex === entry.index ? '▾' : '▸'}
								{entry.body.title || 'Untitled session'}
								<span class="muted">
									· {entry.body.blocks.length} block{entry.body.blocks.length === 1 ? '' : 's'}
								</span>
							</button>
							<div style="flex:0 1 130px">
								<select
									aria-label="Day"
									bind:value={draft.sessions[entry.index].day_of_week}
								>
									{#each DAY_NAMES as name, i (name)}
										<option value={i + 1}>{name}</option>
									{/each}
								</select>
							</div>
							<button class="link" style="flex:0 0 auto" onclick={() => removeSession(entry.index)}>
								Remove
							</button>
						</div>

						{#if openIndex === entry.index}
							<div style="margin-top:0.6rem">
								<SessionEditor body={draft.sessions[entry.index].body} {exercises} />
							</div>
						{/if}
					</div>
				{/each}
			</div>
		{/each}
	</div>

	<div class="row" style="margin-top:1.2rem">
		<button style="flex:0 0 auto" onclick={save} disabled={busy || !draft.title.trim() || sessionCount === 0}>
			{busy ? 'Saving…' : draft.id ? 'Save changes' : 'Save routine'}
		</button>
		<button class="ghost" style="flex:0 0 auto" onclick={() => (draft = null)} disabled={busy}>
			Cancel
		</button>
		<p class="muted" style="flex:1 1 auto;margin:0;text-align:right;font-size:0.82rem">
			{sessionCount} session{sessionCount === 1 ? '' : 's'} in the week
		</p>
	</div>

	{#if draft.id}
		<p class="muted" style="margin-top:0.6rem;font-size:0.82rem">
			Saving rewrites this routine's sessions from today on. Sessions already ticked off, and
			everything before today, are left as they are.
		</p>
	{/if}
{/if}

<style>
	.tag {
		font-family: var(--mono);
		font-size: 0.68rem;
		text-transform: uppercase;
		letter-spacing: 0.1em;
		color: var(--muted);
		border: 1px solid var(--line);
		border-radius: var(--radius);
		padding: 0.1rem 0.4rem;
		margin-left: 0.4rem;
		vertical-align: 0.1rem;
	}
	.tag.on {
		color: var(--signal-text);
		border-color: var(--signal);
	}
	.day {
		border: 1px solid var(--line);
		border-radius: var(--radius);
		padding: 0.7rem 0.8rem;
		background: var(--panel);
	}
	.day.rest {
		background: none;
	}
	.session {
		border-top: 1px solid var(--line);
		margin-top: 0.6rem;
		padding-top: 0.6rem;
	}
	.spacer {
		flex: 1 1 auto;
	}
	button.small {
		padding: 0.4rem 0.7rem;
		font-size: 0.8rem;
	}
</style>
