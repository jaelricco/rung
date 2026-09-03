<script>
	import '../app.css';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { session, loadUser, signOut } from '$lib/session.svelte.js';
	import { theme, initTheme, setTheme } from '$lib/theme.svelte.js';

	let { children } = $props();

	// The destinations, in two groups: what you do this week, and what you set
	// once and revisit. Eight links in a column need the split to stay
	// readable; the order inside each group is the order you meet them in.
	const NAV = [
		{
			group: 'Training',
			items: [
				{ href: '/', label: 'Overview' },
				{ href: '/log', label: 'Log a session' },
				{ href: '/plan', label: 'Plan' },
				{ href: '/calendar', label: 'Calendar' }
			]
		},
		{
			group: 'Setup',
			items: [
				{ href: '/baseline', label: 'Baseline' },
				{ href: '/routine', label: 'Routine' },
				{ href: '/events', label: 'Events' },
				{ href: '/settings', label: 'Settings' }
			]
		}
	];

	const THEMES = [
		{ id: 'system', label: 'Auto' },
		{ id: 'light', label: 'Light' },
		{ id: 'dark', label: 'Dark' }
	];

	// The pages whose main content is a seven-column calendar; they get the
	// wider shell so the grid has room for a day in every column.
	const WIDE_PAGES = new Set(['/calendar', '/plan']);
	let ready = $state(false);

	// Below the breakpoint the navigation is a drawer rather than a column, so
	// it has to be opened, closed, and kept out of the tab order while shut.
	let navOpen = $state(false);
	let narrow = $state(false);

	initTheme();

	$effect(() => {
		const mq = window.matchMedia('(max-width: 900px)');
		const sync = () => (narrow = mq.matches);
		sync();
		mq.addEventListener('change', sync);
		return () => mq.removeEventListener('change', sync);
	});

	// Arriving somewhere is the end of using the drawer.
	$effect(() => {
		if (page.url.pathname) navOpen = false;
	});

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

<svelte:window onkeydown={(e) => e.key === 'Escape' && (navOpen = false)} />

{#snippet body()}
	{#if !ready}
		<p class="eyebrow" style="padding-top:3rem">Loading</p>
	{:else}
		{@render children()}
	{/if}
{/snippet}

{#if session.user}
	<div class="app">
		<a class="skip" href="#page">Skip to the page</a>

		<aside class="sidenav" class:open={navOpen} inert={narrow && !navOpen}>
			<a class="mark" href="/">RIG<span class="sub">Calisthenics log</span></a>

			<nav>
				{#each NAV as section (section.group)}
					<div class="navgroup">
						<p class="eyebrow">{section.group}</p>
						{#each section.items as item (item.href)}
							<a
								href={item.href}
								aria-current={page.url.pathname === item.href ? 'page' : undefined}
							>
								{item.label}
							</a>
						{/each}
					</div>
				{/each}
			</nav>

			<div class="foot">
				<div class="bar" style="margin:0"></div>
				<div class="themepick" role="group" aria-label="Colour palette">
					{#each THEMES as option (option.id)}
						<button
							type="button"
							aria-pressed={theme.choice === option.id}
							onclick={() => setTheme(option.id)}
						>
							{option.label}
						</button>
					{/each}
				</div>
				<p class="who">{session.user.display_name}</p>
				<button class="link" onclick={handleSignOut}>Sign out</button>
			</div>
		</aside>

		{#if narrow}
			<button
				class="nav-scrim"
				class:open={navOpen}
				aria-label="Close navigation"
				onclick={() => (navOpen = false)}
			></button>
		{/if}

		<main id="page">
			<div class="mobilebar">
				<button
					class="navtoggle"
					aria-label="Navigation"
					aria-expanded={navOpen}
					onclick={() => (navOpen = !navOpen)}
				>
					☰
				</button>
				<span class="mark">RIG</span>
			</div>

			<div class="shell" class:wide={WIDE_PAGES.has(page.url.pathname)}>
				{@render body()}
			</div>
		</main>
	</div>
{:else}
	<div class="shell">
		{@render body()}
	</div>
{/if}
