<script>
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { api } from '$lib/api.js';
	import { session } from '$lib/session.svelte.js';

	let mode = $state('signin');
	let email = $state('');
	let password = $state('');
	let displayName = $state('');
	let busy = $state(false);

	// A sign-in that failed at the provider comes back as a redirect carrying
	// its reason, so the page has to read the URL as well as its own state.
	let failed = $state('');
	let error = $derived(failed || page.url.searchParams.get('error') || '');

	// Which providers this server has been given credentials for. None
	// configured means the buttons simply do not appear.
	let providers = $state([]);
	$effect(() => {
		api.get('/auth/providers').then((list) => (providers = list ?? []));
	});

	async function submit() {
		failed = '';
		// Clear a stale reason from a previous redirect before trying again.
		if (page.url.searchParams.has('error')) history.replaceState(null, '', '/login');
		busy = true;
		try {
			const path = mode === 'signin' ? '/auth/login' : '/auth/register';
			const body =
				mode === 'signin'
					? { email, password }
					: { email, password, display_name: displayName };
			session.user = await api.post(path, body);
			goto('/');
		} catch (e) {
			failed = e.message;
		} finally {
			busy = false;
		}
	}
</script>

<div style="max-width:400px;margin:14vh auto 0">
	<p class="eyebrow">Calisthenics training log</p>
	<h1>Train on evidence,<br />not on feel.</h1>
	<div class="bar"></div>

	{#if providers.length}
		{#each providers as provider (provider.id)}
			<a class="provider" href="/api/v1/auth/oauth/{provider.id}/start" data-sveltekit-reload>
				Continue with {provider.label}
			</a>
		{/each}
		<p class="or"><span>or</span></p>
	{/if}

	<div class="field">
		<label for="email">Email</label>
		<input id="email" type="email" bind:value={email} autocomplete="email" />
	</div>

	{#if mode === 'register'}
		<div class="field">
			<label for="name">Name</label>
			<input id="name" type="text" bind:value={displayName} autocomplete="name" />
		</div>
	{/if}

	<div class="field">
		<label for="password">Password</label>
		<input
			id="password"
			type="password"
			bind:value={password}
			autocomplete={mode === 'signin' ? 'current-password' : 'new-password'}
			onkeydown={(e) => e.key === 'Enter' && submit()}
		/>
		{#if mode === 'register'}
			<p class="muted" style="font-size:0.8rem;margin:0.4rem 0 0">At least 10 characters.</p>
		{/if}
	</div>

	{#if error}
		<div class="notice error" style="margin-bottom:0.9rem">{error}</div>
	{/if}

	<button onclick={submit} disabled={busy} style="width:100%">
		{busy ? 'Working' : mode === 'signin' ? 'Sign in' : 'Create account'}
	</button>

	<p style="margin-top:1rem;font-size:0.88rem" class="muted">
		{#if mode === 'signin'}
			No account yet?
			<button class="link" onclick={() => ((mode = 'register'), (failed = ''))}>Create one</button>
		{:else}
			Already have an account?
			<button class="link" onclick={() => ((mode = 'signin'), (failed = ''))}>Sign in</button>
		{/if}
	</p>
</div>

<style>
	/* A provider button is a link, because it leaves the app entirely: the
	   flow is two browser redirects, not a fetch. */
	.provider {
		display: block;
		text-align: center;
		text-decoration: none;
		font-family: var(--display);
		font-weight: 600;
		font-size: 0.9rem;
		color: var(--chalk);
		border: 1px solid var(--line);
		border-radius: var(--radius);
		padding: 0.6rem 1.1rem;
		margin-bottom: 0.6rem;
	}
	.provider:hover {
		border-color: var(--signal);
	}
	.or {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		margin: 1.1rem 0;
		color: var(--muted);
		font-family: var(--mono);
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.12em;
	}
	.or::before,
	.or::after {
		content: '';
		flex: 1;
		height: 1px;
		background: var(--line);
	}
</style>
