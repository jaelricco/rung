<script>
	import '../app.css';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { session, loadUser, signOut } from '$lib/session.svelte.js';

	let { children } = $props();
	let ready = $state(false);

	$effect(() => {
		if (ready) return;
		loadUser().then(() => {
			ready = true;
			if (!session.user && page.url.pathname !== '/login') {
				goto('/login', { replaceState: true });
			}
		});
	});

	async function handleSignOut() {
		await signOut();
		goto('/login');
	}
</script>

<div class="shell">
	{#if session.user}
		<header class="topbar">
			<span class="mark">RIG</span>
			<nav>
				<a href="/" aria-current={page.url.pathname === '/' ? 'page' : undefined}>Overview</a>
				<a href="/log" aria-current={page.url.pathname === '/log' ? 'page' : undefined}>Log a session</a>
				<a href="/plan" aria-current={page.url.pathname === '/plan' ? 'page' : undefined}>Plan</a>
				<a href="/events" aria-current={page.url.pathname === '/events' ? 'page' : undefined}>Events</a>
			</nav>
			<span class="spacer"></span>
			<button class="link" onclick={handleSignOut}>Sign out</button>
		</header>
	{/if}

	{#if !ready}
		<p class="eyebrow" style="padding-top:3rem">Loading</p>
	{:else}
		{@render children()}
	{/if}
</div>
