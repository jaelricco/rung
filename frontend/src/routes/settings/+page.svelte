<script>
	import { api } from '$lib/api.js';
	import Failure from '$lib/Failure.svelte';

	let account = $state(null); // { connected, connection, providers, keystore_ready }
	let error = $state(null);
	let busy = $state(false);
	let saved = $state('');

	let provider = $state('anthropic');
	let apiKey = $state('');
	let model = $state('');

	let providers = $derived(account?.providers ?? []);
	let chosen = $derived(providers.find((p) => p.id === provider) ?? null);
	let connection = $derived(account?.connection ?? null);

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
		} catch (e) {
			error = e;
		}
	}

	$effect(() => {
		if (!account) load();
	});

	// Switching the provider switches the model with it: a Claude model name
	// means nothing to OpenAI.
	function pickProvider(id) {
		provider = id;
		const p = providers.find((x) => x.id === id);
		if (p) model = p.default_model;
	}

	async function save() {
		busy = true;
		error = null;
		saved = '';
		try {
			account = await api.put('/me/ai', { provider, api_key: apiKey, model });
			apiKey = '';
			saved = 'Connected. The key was checked against the provider before it was stored.';
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
</script>

<p class="eyebrow" style="margin-top:2rem">Settings</p>
<h1>Your AI account</h1>

<div class="column form-width">
	<p class="muted" style="line-height:1.6">
		Plans, reviews, recovery guidance and event search all run on a model. They run on
		<em>your</em> model account, not this server's: connect a key from Anthropic or OpenAI, and this
		app spends it only on the requests you make. Your provider bills you directly for what you
		used, and no other athlete's requests ever touch your key.
	</p>

	{#if account && !account.keystore_ready}
		<div class="notice error">
			This server has no AI_CREDENTIALS_KEY set, so it cannot store keys safely yet. Nothing can be
			connected until the operator sets one.
		</div>
	{/if}

	{#if connection}
		<div class="panel">
			<p class="eyebrow">Connected</p>
			<p style="margin:0.3rem 0 0.1rem">
				<strong>{connection.label || connection.provider}</strong>
				<span class="mono muted"> · {connection.model}</span>
			</p>
			<p class="mono muted" style="font-size:0.8rem">
				key ••••{connection.key_hint} · added {new Date(connection.updated_at).toLocaleDateString()}
				{#if connection.last_used_at}
					· last used {new Date(connection.last_used_at).toLocaleDateString()}
				{/if}
			</p>
			<button class="ghost" style="margin-top:0.8rem" onclick={disconnect} disabled={busy}>
				Remove this key
			</button>
		</div>
	{:else if account}
		<div class="empty">
			No AI account connected yet. The coaching features stay off until you add a key below.
		</div>
	{/if}

	<div class="panel" style="margin-top:1rem">
		<p class="eyebrow">{connection ? 'Replace the key' : 'Connect an account'}</p>

		<div class="field">
			<label for="provider">Provider</label>
			<select id="provider" value={provider} onchange={(e) => pickProvider(e.currentTarget.value)}>
				{#each providers as p (p.id)}
					<option value={p.id}>{p.label}</option>
				{/each}
			</select>
		</div>

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
				<p class="mono muted" style="font-size:0.75rem;margin:0.4rem 0 0">
					Make one at <a href={chosen.keys_url} target="_blank" rel="noreferrer noopener"
						>{chosen.keys_url}</a
					>. It is stored encrypted and never shown back to you.
				</p>
			{/if}
		</div>

		<div class="field">
			<label for="model">Model</label>
			<input id="model" list="models" bind:value={model} />
			<datalist id="models">
				{#each chosen?.models ?? [] as m (m)}
					<option value={m}></option>
				{/each}
			</datalist>
			<p class="mono muted" style="font-size:0.75rem;margin:0.4rem 0 0">
				The first is the strongest and the most expensive. A cheaper one writes plans faster and
				costs you less.
			</p>
		</div>

		<button onclick={save} disabled={busy || !account?.keystore_ready}>
			{busy ? 'Checking with the provider' : connection && !apiKey ? 'Save model' : 'Connect'}
		</button>
	</div>

	{#if saved}
		<div class="notice" style="margin-top:1rem">{saved}</div>
	{/if}
	<Failure {error} />

	<p class="muted" style="font-size:0.85rem;line-height:1.6;margin-top:1.5rem">
		Web search bills separately at both providers, and both the plan writer and the event search use
		it. Turn research off on the plan page for a cheaper run.
	</p>
</div>
