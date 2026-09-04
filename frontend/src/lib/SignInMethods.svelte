<script>
	import { page } from '$app/state';
	import { api } from '$lib/api.js';

	// How this athlete gets into their own account. Nothing here touches the
	// AI features: signing in with a provider says who you are, and neither
	// Anthropic nor OpenAI lets a site spend a signed-in consumer account's
	// plan, so the API key below stays a separate thing.
	let methods = $state(null); // { has_password, identities, available }
	let error = $state('');
	let saved = $state('');
	let busy = $state(false);

	let password = $state('');
	let currentPassword = $state('');
	let showPasswordForm = $state(false);

	// A link that failed comes back as a redirect carrying its reason.
	let redirectError = $derived(page.url.searchParams.get('error') ?? '');

	let unlinked = $derived(
		(methods?.available ?? []).filter(
			(p) => !(methods?.identities ?? []).some((i) => i.provider === p.id)
		)
	);
	// Removing the last way in would lock the athlete out of the account.
	let onlyWayIn = $derived(!methods?.has_password && (methods?.identities?.length ?? 0) <= 1);

	async function load() {
		try {
			methods = await api.get('/me/logins');
		} catch (e) {
			error = e.message;
		}
	}

	$effect(() => {
		if (!methods) load();
	});

	async function unlink(provider) {
		busy = true;
		error = '';
		saved = '';
		try {
			await api.del(`/me/logins/${provider}`);
			methods = null;
			await load();
			saved = 'Unlinked.';
		} catch (e) {
			error = e.message;
		} finally {
			busy = false;
		}
	}

	async function savePassword() {
		busy = true;
		error = '';
		saved = '';
		try {
			await api.put('/me/password', {
				current_password: currentPassword,
				password
			});
			password = '';
			currentPassword = '';
			showPasswordForm = false;
			methods = null;
			await load();
			saved = 'Password saved.';
		} catch (e) {
			error = e.message;
		} finally {
			busy = false;
		}
	}
</script>

<div class="panel">
	<p class="eyebrow">How you sign in</p>

	{#if methods}
		<ul class="methods">
			{#if methods.has_password}
				<li>
					<span>Email and password</span>
					<span class="mono muted">set</span>
				</li>
			{/if}
			{#each methods.identities as identity (identity.provider)}
				<li>
					<span>{identity.label}<span class="mono muted"> · {identity.email}</span></span>
					<button
						class="link"
						onclick={() => unlink(identity.provider)}
						disabled={busy || onlyWayIn}
						title={onlyWayIn ? 'This is the only way into your account.' : ''}
					>
						Unlink
					</button>
				</li>
			{/each}
		</ul>

		{#if !methods.has_password && methods.identities.length <= 1}
			<p class="muted" style="font-size:0.85rem;margin:0 0 0.8rem">
				This is currently the only way into your account. Set a password, or link a second
				provider, so losing access to it doesn't lose you the account.
			</p>
		{/if}

		{#each unlinked as provider (provider.id)}
			<a class="ghost-link" href="/api/v1/auth/oauth/{provider.id}/start" data-sveltekit-reload>
				Link {provider.label}
			</a>
		{/each}

		{#if showPasswordForm}
			{#if methods.has_password}
				<div class="field" style="margin-top:0.9rem">
					<label for="current">Current password</label>
					<input id="current" type="password" bind:value={currentPassword} autocomplete="current-password" />
				</div>
			{/if}
			<div class="field">
				<label for="new">{methods.has_password ? 'New password' : 'Password'}</label>
				<input id="new" type="password" bind:value={password} autocomplete="new-password" />
				<p class="mono muted" style="font-size:0.75rem;margin:0.4rem 0 0">At least 10 characters.</p>
			</div>
			<button onclick={savePassword} disabled={busy}>
				{busy ? 'Saving' : 'Save password'}
			</button>
			<button class="link" style="margin-left:0.8rem" onclick={() => (showPasswordForm = false)}>
				Cancel
			</button>
		{:else}
			<button class="ghost" style="margin-top:0.3rem" onclick={() => (showPasswordForm = true)}>
				{methods.has_password ? 'Change password' : 'Set a password'}
			</button>
		{/if}
	{:else if !error}
		<p class="muted">Loading</p>
	{/if}

	{#if saved}
		<p class="notice" style="margin-top:0.9rem">{saved}</p>
	{/if}
	{#if error || redirectError}
		<p class="notice error" style="margin-top:0.9rem">{error || redirectError}</p>
	{/if}
</div>

<style>
	.methods {
		list-style: none;
		padding: 0;
		margin: 0.4rem 0 0.9rem;
	}
	.methods li {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 0.8rem;
		padding: 0.45rem 0;
		border-bottom: 1px solid var(--line);
		font-size: 0.92rem;
	}
	.methods li:last-child {
		border-bottom: none;
	}
	/* Leaves the app entirely — the flow is two browser redirects, not a fetch. */
	.ghost-link {
		display: inline-block;
		text-decoration: none;
		font-family: var(--sans);
		font-weight: 600;
		font-size: 0.9rem;
		color: var(--chalk);
		border: 1px solid var(--line);
		border-radius: var(--radius);
		padding: 0.6rem 1.1rem;
		margin: 0 0.6rem 0.6rem 0;
	}
	.ghost-link:hover {
		border-color: var(--signal);
	}
</style>
