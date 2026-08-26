<script>
	import { goto } from '$app/navigation';
	import { api } from '$lib/api.js';
	import { session } from '$lib/session.svelte.js';

	let mode = $state('signin');
	let email = $state('');
	let password = $state('');
	let displayName = $state('');
	let error = $state('');
	let busy = $state(false);

	async function submit() {
		error = '';
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
			error = e.message;
		} finally {
			busy = false;
		}
	}
</script>

<div style="max-width:400px;margin:14vh auto 0">
	<p class="eyebrow">Calisthenics training log</p>
	<h1>Train on evidence,<br />not on feel.</h1>
	<div class="bar"></div>

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
			<button class="link" onclick={() => ((mode = 'register'), (error = ''))}>Create one</button>
		{:else}
			Already have an account?
			<button class="link" onclick={() => ((mode = 'signin'), (error = ''))}>Sign in</button>
		{/if}
	</p>
</div>
