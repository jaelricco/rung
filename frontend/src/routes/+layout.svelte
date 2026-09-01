<script>
	import '../app.css';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { session, loadUser, signOut } from '$lib/session.svelte.js';

	let { children } = $props();

	// The pages whose main content is a seven-column calendar; they get the
	// wider shell so the grid has room for a day in every column.
	const WIDE_PAGES = new Set(['/calendar', '/plan']);
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

<div class="shell" class:wide={WIDE_PAGES.has(page.url.pathname)}>
	{#if session.user}
		<header class="topbar">
			<span class="mark">RIG</span>
			<nav>
				<a href="/" aria-current={page.url.pathname === '/' ? 'page' : undefined}>Overview</a>
				<a href="/log" aria-current={page.url.pathname === '/log' ? 'page' : undefined}>Log a session</a>
				<a href="/plan" aria-current={page.url.pathname === '/plan' ? 'page' : undefined}>Plan</a>
				<a
					href="/baseline"
					aria-current={page.url.pathname === '/baseline' ? 'page' : undefined}>Baseline</a
				>
				<a href="/routine" aria-current={page.url.pathname === '/routine' ? 'page' : undefined}>
					Routine
				</a>
				<a href="/calendar" aria-current={page.url.pathname === '/calendar' ? 'page' : undefined}>
					Calendar
				</a>
				<a href="/events" aria-current={page.url.pathname === '/events' ? 'page' : undefined}>Events</a>
				<a href="/settings" aria-current={page.url.pathname === '/settings' ? 'page' : undefined}>
					Settings
				</a>
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
