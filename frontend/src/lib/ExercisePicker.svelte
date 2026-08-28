<script>
	// The library is 139 movements deep, which is past the point where a plain
	// select is usable: finding "rings-turned-out push-up" meant scrolling a
	// list of everything. This keeps the behaviour a select has — click the
	// field, see the whole library grouped by category — and adds the one it
	// lacks: type and the list narrows to what matches.
	let { value = $bindable(''), exercises = [], id = '', onselect } = $props();

	let open = $state(false);
	let query = $state('');
	let active = $state(0);
	let input = $state(null);
	let list = $state(null);

	let selected = $derived(exercises.find((e) => e.slug === value) ?? null);
	// The field reads as the chosen exercise until it is being typed in.
	let label = $derived(selected?.name ?? value ?? '');

	// Ranked rather than merely filtered: a name that starts with what was
	// typed is what was meant, and "pull" should not surface a description
	// mentioning pulling before the pull-up itself.
	function score(exercise, needle) {
		const name = exercise.name.toLowerCase();
		const slug = exercise.slug.toLowerCase();
		if (name.startsWith(needle)) return 0;
		if (name.includes(` ${needle}`)) return 1;
		if (name.includes(needle)) return 2;
		if (slug.includes(needle)) return 3;
		if (exercise.category.toLowerCase().startsWith(needle)) return 4;
		if ((exercise.description ?? '').toLowerCase().includes(needle)) return 5;
		return -1;
	}

	let needle = $derived(query.trim().toLowerCase());
	let searching = $derived(needle.length > 0);

	let matches = $derived.by(() => {
		if (!searching) return exercises;
		return exercises
			.map((exercise) => ({ exercise, rank: score(exercise, needle) }))
			.filter((m) => m.rank >= 0)
			.sort((a, b) => a.rank - b.rank || a.exercise.difficulty - b.exercise.difficulty)
			.map((m) => m.exercise);
	});

	// Category headings are worth their space while browsing the whole library
	// and only get in the way once a search has reordered it.
	let rows = $derived.by(() => {
		const out = [];
		let category = null;
		for (const exercise of matches) {
			if (!searching && exercise.category !== category) {
				category = exercise.category;
				out.push({ heading: category });
			}
			out.push({ exercise });
		}
		return out;
	});

	let options = $derived(rows.filter((row) => row.exercise).map((row) => row.exercise));

	function show() {
		open = true;
		query = '';
		active = Math.max(0, options.findIndex((e) => e.slug === value));
		// Open on what is already chosen rather than at the top of 139 rows,
		// so the field reads as "here is what you picked, in context".
		scrollActiveIntoView();
	}

	function close() {
		open = false;
		query = '';
	}

	function choose(exercise) {
		if (!exercise) return;
		value = exercise.slug;
		onselect?.(exercise);
		close();
		input?.focus();
	}

	function onKeyDown(event) {
		if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
			event.preventDefault();
			if (!open) {
				show();
				return;
			}
			const step = event.key === 'ArrowDown' ? 1 : -1;
			active = (active + step + options.length) % options.length;
			scrollActiveIntoView();
			return;
		}
		if (event.key === 'Enter') {
			if (open) {
				event.preventDefault();
				choose(options[active]);
			}
			return;
		}
		if (event.key === 'Escape' && open) {
			event.preventDefault();
			close();
			return;
		}
		if (event.key === 'Tab') close();
	}

	function scrollActiveIntoView() {
		// $effect would fire after the DOM updates, but the list is only ever
		// moved by these two keys, so doing it here keeps it obvious.
		queueMicrotask(() => {
			list?.querySelector('[data-active="true"]')?.scrollIntoView({ block: 'nearest' });
		});
	}

	function onInput(event) {
		query = event.currentTarget.value;
		open = true;
		active = 0;
	}
</script>

<div class="picker">
	<input
		{id}
		bind:this={input}
		type="text"
		role="combobox"
		aria-expanded={open}
		aria-controls={`${id}-list`}
		aria-activedescendant={open && options[active] ? `${id}-option-${active}` : undefined}
		aria-autocomplete="list"
		autocomplete="off"
		spellcheck="false"
		placeholder="Type to search the library"
		value={open ? query : label}
		oninput={onInput}
		onfocus={show}
		onmousedown={() => (open || show())}
		onblur={close}
		onkeydown={onKeyDown}
	/>

	{#if open}
		<!-- pointerdown is swallowed so the input keeps focus: losing it would
		     close the list before the click lands on an option. -->
		<div
			class="list"
			id={`${id}-list`}
			role="listbox"
			tabindex="-1"
			bind:this={list}
			onpointerdown={(event) => event.preventDefault()}
		>
			{#each rows as row, index (row.heading ?? row.exercise.slug)}
				{#if row.heading}
					<p class="heading">{row.heading}</p>
				{:else}
					{@const position = options.indexOf(row.exercise)}
					<button
						type="button"
						role="option"
						id={`${id}-option-${position}`}
						class="option"
						aria-selected={row.exercise.slug === value}
						data-active={position === active}
						onmouseenter={() => (active = position)}
						onclick={() => choose(row.exercise)}
					>
						<span class="name">{row.exercise.name}</span>
						<span class="meta">
							{#if searching}{row.exercise.category} · {/if}level {row.exercise.difficulty}
						</span>
					</button>
				{/if}
			{:else}
				<p class="heading">Nothing in the library matches that.</p>
			{/each}
		</div>
	{/if}
</div>

<style>
	.picker {
		position: relative;
	}
	.list {
		position: absolute;
		z-index: 20;
		top: calc(100% + 2px);
		left: 0;
		right: 0;
		max-height: 17rem;
		overflow-y: auto;
		background: var(--panel-lift);
		border: 1px solid var(--line);
		border-radius: var(--radius);
		box-shadow: 0 10px 24px rgba(0, 0, 0, 0.45);
	}
	.heading {
		font-family: var(--mono);
		font-size: 0.68rem;
		text-transform: uppercase;
		letter-spacing: 0.12em;
		color: var(--muted);
		margin: 0;
		padding: 0.5rem 0.7rem 0.25rem;
		position: sticky;
		top: 0;
		background: var(--panel-lift);
	}
	.option {
		display: flex;
		align-items: baseline;
		gap: 0.6rem;
		width: 100%;
		background: none;
		border: none;
		border-radius: 0;
		text-align: left;
		padding: 0.4rem 0.7rem;
		color: var(--chalk);
		font-family: var(--display);
		font-weight: 400;
		font-size: 0.9rem;
		letter-spacing: 0;
	}
	.option[data-active='true'] {
		background: var(--panel);
		filter: none;
	}
	.option[aria-selected='true'] .name {
		color: var(--signal);
	}
	.option .name {
		flex: 1 1 auto;
	}
	.option .meta {
		font-family: var(--mono);
		font-size: 0.72rem;
		color: var(--muted);
		flex: 0 0 auto;
	}
</style>
