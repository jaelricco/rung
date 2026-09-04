<script>
	import { api } from '$lib/api.js';
	import Failure from '$lib/Failure.svelte';

	// The AI connector: which provider account this athlete's coaching runs on.
	// Laid out as a card with the providers as tiles, because picking between
	// two named products is a choice you make by looking, not by opening a
	// dropdown and reading model strings.
	let account = $state(null); // { connected, connection, providers, keystore_ready }
	let usage = $state(null); // { month, total, since }
	let error = $state(null);
	let busy = $state(false);
	let saved = $state('');
	let open = $state(true);

	let provider = $state('anthropic');
	let apiKey = $state('');
	let model = $state('');
	let typedModel = $state(false);

	const OTHER_MODEL = '__other__';

	let providers = $derived(account?.providers ?? []);
	let chosen = $derived(providers.find((p) => p.id === provider) ?? null);
	let connection = $derived(account?.connection ?? null);

	// The provider's own list, plus whatever is already connected: an account
	// may be on a model this build has never heard of, and opening the list
	// should not silently move it off that.
	let modelChoices = $derived.by(() => {
		const listed = chosen?.models ?? [];
		return model && !listed.includes(model) ? [...listed, model] : listed;
	});

	async function load() {
		try {
			const next = await api.get('/me/ai');
			if (next.connection) {
				provider = next.connection.provider;
				model = next.connection.model;
			} else if (next.providers?.length) {
				provider = next.providers[0].id;
				model = next.providers[0].default_model;
			}
			account = next;
			// What it has actually cost so far, which is the question anyone
			// asks before pasting a key. Only worth asking once connected.
			if (next.connected) {
				usage = await api.get('/me/ai/usage').catch(() => null);
			}
		} catch (e) {
			error = e;
		}
	}

	// "about $0.31", or "at least" when a model in the window has no published
	// price and the figure is therefore incomplete.
	function spent(window) {
		if (!window?.estimated) return null;
		return `${window.priced ? 'about' : 'at least'} ${window.estimated}`;
	}

	$effect(() => {
		if (!account) load();
	});

	// Switching provider switches the model with it: a Claude model name means
	// nothing to OpenAI.
	function pickProvider(id) {
		provider = id;
		typedModel = false;
		saved = '';
		error = null;
		const p = providers.find((x) => x.id === id);
		if (p) model = p.default_model;
	}

	function pickModel(value) {
		if (value === OTHER_MODEL) {
			typedModel = true;
			model = '';
			return;
		}
		model = value;
	}

	async function save() {
		busy = true;
		error = null;
		saved = '';
		try {
			account = await api.put('/me/ai', { provider, api_key: apiKey, model });
			apiKey = '';
			saved = 'Connected. Your key was checked with the provider before it was stored.';
		} catch (e) {
			error = e;
		} finally {
			busy = false;
		}
	}

	// The two switches on an existing connection. Neither touches the key and
	// neither calls the provider, so they answer immediately.
	async function flip(change, note) {
		busy = true;
		error = null;
		saved = '';
		try {
			account = await api.patch('/me/ai', change);
			saved = note;
		} catch (e) {
			error = e;
		} finally {
			busy = false;
		}
	}

	async function disconnect() {
		busy = true;
		error = null;
		saved = '';
		try {
			account = await api.del('/me/ai');
			apiKey = '';
			saved = 'Key removed. Revoke it at your provider too if you are done with it.';
		} catch (e) {
			error = e;
		} finally {
			busy = false;
		}
	}

	function shortDate(value) {
		return value ? new Date(value).toLocaleDateString() : '';
	}
</script>

<section class="card">
	<header>
		<span class="mark" aria-hidden="true">✳</span>
		<h2>AI connector</h2>
		<button
			class="chevron"
			onclick={() => (open = !open)}
			aria-expanded={open}
			aria-label={open ? 'Collapse' : 'Expand'}
		>
			{open ? '⌃' : '⌄'}
		</button>
	</header>

	{#if open}
		<p class="lede">
			What leaves this server is what the coach has to reason about: your logged sets and records,
			any open injury, your bodyweight, and the skill and notes you type into a request. Your email
			address, your password and your calendar stay here. You are not anonymous to the provider —
			the request is made on your own account, and they bill you for it.
		</p>

		{#if account && !account.keystore_ready}
			<p class="notice error">
				This server can't store keys safely right now, so nothing can be connected. That's a fault
				on our side, not yours.
			</p>
		{/if}

		{#if connection}
			<div class="connected" class:off={connection.paused}>
				<div>
					<strong>{connection.label || connection.provider}</strong>
					<span class="mono muted"> · {connection.model}</span>
					{#if connection.paused}<span class="badge">off</span>{/if}
					<p class="mono muted meta">
						key ••••{connection.key_hint} · added {shortDate(connection.updated_at)}
						{#if connection.last_used_at}· last used {shortDate(connection.last_used_at)}{/if}
					</p>
				</div>
				<button class="ghost" onclick={disconnect} disabled={busy}>Disconnect</button>
			</div>

			<div class="switches">
				<label class="switch">
					<input
						type="checkbox"
						checked={connection.paused}
						disabled={busy}
						onchange={(e) =>
							flip(
								{ paused: e.currentTarget.checked },
								e.currentTarget.checked
									? 'Connector switched off. Your key stays here; nothing is sent to a model.'
									: 'Connector switched back on.'
							)}
					/>
					<span>
						<strong>Switch the connector off</strong>
						<span class="mono muted hint">
							Keeps the key, spends nothing. Plans keep being written from your own records —
							they just do not get a model's pass over them.
						</span>
					</span>
				</label>

				<label class="switch">
					<input
						type="checkbox"
						checked={connection.forget_on_logout}
						disabled={busy}
						onchange={(e) =>
							flip(
								{ forget_on_logout: e.currentTarget.checked },
								e.currentTarget.checked
									? 'This key will be deleted when you sign out.'
									: 'The key will stay until you disconnect it.'
							)}
					/>
					<span>
						<strong>Forget the key when I sign out</strong>
						<span class="mono muted hint">
							For a shared or borrowed machine. It applies when you sign out yourself — a session
							that simply expires leaves the key in place.
						</span>
					</span>
				</label>
			</div>

			{#if usage && usage.total.calls > 0}
				<dl class="usage">
					<div>
						<dt>This month</dt>
						<dd>
							{usage.month.calls}
							{usage.month.calls === 1 ? 'request' : 'requests'}
							{#if spent(usage.month)}· {spent(usage.month)}{/if}
						</dd>
					</div>
					<div>
						<dt>{usage.since ? `Since ${shortDate(usage.since)}` : 'All time'}</dt>
						<dd>
							{usage.total.calls}
							{usage.total.calls === 1 ? 'request' : 'requests'}
							{#if spent(usage.total)}· {spent(usage.total)}{/if}
						</dd>
					</div>
				</dl>
				<p class="note hint">
					Estimated from published rates, from the tokens this app recorded. The figure that
					counts is on your provider's own billing page.
				</p>
			{/if}
		{/if}

		<p class="eyebrow step">{connection ? 'Replace the connection' : 'Choose a provider'}</p>

		<div class="tiles">
			{#each providers as p (p.id)}
				<button class="tile" class:on={provider === p.id} onclick={() => pickProvider(p.id)}>
					<span class="glyph" aria-hidden="true">{p.id === 'anthropic' ? '✳' : '◍'}</span>
					<span class="names">
						<strong class="item-title">{p.label}</strong>
						<span class="vendor">{p.vendor}</span>
					</span>
					{#if provider === p.id}<span class="tick" aria-hidden="true">✓</span>{/if}
				</button>
			{/each}
		</div>

		{#if chosen}
			<p class="caption">
				Runs on your own {chosen.vendor} account, and is billed to you by {chosen.vendor}.
				<strong>A {chosen.label} subscription does not pay for this</strong> — {chosen.vendor} bills
				API use separately from it. A small top-up covers months of ordinary use, and the
				cheapest model in the list is a fraction of the dearest.
			</p>
		{/if}

		<div class="field">
			<label for="key">API key</label>
			<input
				id="key"
				type="password"
				bind:value={apiKey}
				autocomplete="off"
				placeholder={chosen ? `${chosen.key_prefix}…` : ''}
			/>
			{#if chosen}
				<p class="note hint">
					Make one at <a class="mono" href={chosen.keys_url} target="_blank" rel="noreferrer noopener"
						>{chosen.keys_url}</a
					>. Stored encrypted, and never shown back to you.
				</p>
			{/if}
		</div>

		<div class="field">
			<label for="model">Model</label>
			{#if typedModel}
				<input id="model" bind:value={model} placeholder="model name" autocomplete="off" />
				<p class="note hint">
					Any model your account can reach; it is checked before anything is saved.
					<button class="link" onclick={() => ((typedModel = false), (model = chosen?.default_model ?? ''))}>
						Back to the list
					</button>
				</p>
			{:else}
				<select id="model" value={model} onchange={(e) => pickModel(e.currentTarget.value)}>
					{#each modelChoices as m (m)}
						<option value={m}>{m}</option>
					{/each}
					<option value={OTHER_MODEL}>Another model…</option>
				</select>
				<p class="note hint">
					Cheapest first, dearest last. The default is sized for what the model does here — improve
					a plan the app has already written — not for writing one from nothing.
				</p>
			{/if}
		</div>

		<button
			onclick={save}
			disabled={busy || !account?.keystore_ready}
			title={account && !account.keystore_ready
				? "This server can't store keys right now, so there is nothing to connect to."
				: ''}
		>
			{busy
				? 'Checking with the provider'
				: account && !account.keystore_ready
					? 'Unavailable'
					: connection
						? 'Save'
						: 'Connect'}
		</button>

		{#if saved}<p class="notice ok">{saved}</p>{/if}
		<Failure {error} style="margin-top:0.9rem" />

		<p class="foot muted">
			You do not have to connect anything. Without a key the app still writes your training plans
			itself, from your own records, and logging, the calendar, your routines and the baseline all
			work unchanged. A key buys four things on top: the model's pass over the plan, the four-week
			review, the recovery guidance, and the event search.
		</p>
	{/if}
</section>

<style>
	.card {
		border: 1px solid var(--line);
		border-radius: var(--radius);
		background: var(--panel);
		padding: 1.1rem 1.2rem 1.3rem;
	}
	header {
		display: flex;
		align-items: center;
		gap: 0.7rem;
	}
	h2 {
		font-size: 1.15rem;
		margin: 0;
		flex: 1;
	}
	.mark {
		display: grid;
		place-items: center;
		width: 2rem;
		height: 2rem;
		border-radius: var(--radius);
		background: var(--panel-lift);
		color: var(--signal-text);
	}
	.chevron {
		background: none;
		border: none;
		color: var(--muted);
		font-size: 1.1rem;
		padding: 0.2rem 0.4rem;
		cursor: pointer;
	}
	.lede {
		color: var(--muted);
		font-size: 0.95rem;
		line-height: 1.6;
	}
	.step {
		margin: 1.4rem 0 0.6rem;
	}
	.connected {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		flex-wrap: wrap;
		border: 1px solid var(--line);
		border-radius: var(--radius);
		background: var(--panel-lift);
		padding: 0.8rem 0.9rem;
		margin-top: 1rem;
	}
	.connected.off {
		border-style: dashed;
		opacity: 0.75;
	}
	.badge {
		font-family: var(--mono);
		font-size: 0.65rem;
		text-transform: uppercase;
		letter-spacing: 0.12em;
		border: 1px solid var(--line);
		border-radius: 999px;
		padding: 0.1rem 0.45rem;
		margin-left: 0.4rem;
		color: var(--muted);
		vertical-align: middle;
	}
	.meta {
		font-size: 0.9rem;
		margin: 0.25rem 0 0;
	}

	/* The two switches on an existing connection, below the connection they
	   act on rather than down with the fields that replace it. */
	.switches {
		display: grid;
		gap: 0.55rem;
		margin-top: 0.7rem;
	}
	.switch {
		display: flex;
		align-items: flex-start;
		gap: 0.65rem;
		cursor: pointer;
	}
	.switch input {
		width: auto;
		margin: 0.15rem 0 0;
		flex: none;
		accent-color: var(--signal);
	}
	.switch > span {
		display: flex;
		flex-direction: column;
		gap: 0.15rem;
	}
	.switch strong {
		font-weight: 500;
		font-size: 0.9rem;
	}
	.switch .hint {
		margin: 0;
		line-height: 1.5;
	}

	/* The providers, as things you point at rather than names in a list. */
	.tiles {
		display: grid;
		gap: 0.6rem;
	}
	.tile {
		display: flex;
		align-items: center;
		gap: 0.85rem;
		width: 100%;
		text-align: left;
		background: var(--panel-lift);
		border: 1px solid var(--line);
		border-radius: var(--radius);
		padding: 0.8rem 0.9rem;
		color: var(--chalk);
		font-family: var(--sans);
		cursor: pointer;
	}
	.tile:hover {
		border-color: var(--muted);
		filter: none;
	}
	.tile.on {
		border-color: var(--signal);
		box-shadow: inset 0 0 0 1px var(--signal);
	}
	.glyph {
		display: grid;
		place-items: center;
		width: 2.1rem;
		height: 2.1rem;
		border-radius: var(--radius);
		background: var(--panel);
		color: var(--signal-text);
		font-size: 1.05rem;
	}
	.names {
		display: flex;
		flex-direction: column;
		line-height: 1.25;
		flex: 1;
	}
	.vendor {
		font-family: var(--mono);
		font-size: 0.82rem;
		color: var(--muted);
	}
	.tick {
		color: var(--signal-text);
		font-size: 1.1rem;
	}
	.caption {
		color: var(--muted);
		font-size: 0.95rem;
		margin: 0.6rem 0 1.2rem;
	}
	.hint {
		font-size: 0.85rem;
		margin: 0.4rem 0 0;
	}
	.usage {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(11rem, 1fr));
		gap: 0.6rem;
		margin: 0.7rem 0 0;
	}
	.usage div {
		border: 1px solid var(--line);
		border-radius: var(--radius);
		padding: 0.6rem 0.7rem;
	}
	.usage dt {
		font-family: var(--mono);
		font-size: 0.76rem;
		text-transform: uppercase;
		letter-spacing: 0.12em;
		color: var(--muted);
	}
	.usage dd {
		margin: 0.25rem 0 0;
		font-family: var(--mono);
		font-size: 0.95rem;
	}
	.notice.ok {
		margin-top: 0.9rem;
	}
	.foot {
		font-size: 0.9rem;
		line-height: 1.6;
		margin: 1.4rem 0 0;
	}
</style>
