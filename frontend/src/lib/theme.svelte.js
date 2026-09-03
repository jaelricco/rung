// Which palette the interface is drawn in. Three choices, not two: a gym at
// six in the morning wants the dark one, a desk by a window wants the light
// one, and most people would rather their machine decide. The choice is kept
// in localStorage under one key, and the resolved answer is written onto
// <html> as data-theme, which is the only thing the stylesheet reads.

const KEY = 'rig-theme';
const CHOICES = ['system', 'light', 'dark'];
const DARK = '(prefers-color-scheme: dark)';

// The background of each palette, so the browser chrome — the address bar on
// a phone, the area behind an overscroll — matches the page.
const THEME_COLOR = { light: '#f5f6f3', dark: '#0a1012' };

export const theme = $state({ choice: 'system', resolved: 'dark' });

// initTheme is called from the layout, which mounts once — except under a dev
// server, where it mounts again on every reload and would stack up listeners.
let started = false;

function systemTheme() {
	if (typeof window === 'undefined' || !window.matchMedia) return 'dark';
	return window.matchMedia(DARK).matches ? 'dark' : 'light';
}

function stored() {
	try {
		const value = localStorage.getItem(KEY);
		return CHOICES.includes(value) ? value : 'system';
	} catch {
		// Private browsing, or storage turned off. Follow the system instead.
		return 'system';
	}
}

function apply() {
	theme.resolved = theme.choice === 'system' ? systemTheme() : theme.choice;
	if (typeof document === 'undefined') return;
	document.documentElement.dataset.theme = theme.resolved;
	const meta = document.querySelector('meta[name="theme-color"]');
	if (meta) meta.setAttribute('content', THEME_COLOR[theme.resolved]);
}

// Called once, from the layout. The inline script in app.html has already
// painted the right palette; this reads the same key so the control in the
// sidebar shows what is actually set.
export function initTheme() {
	theme.choice = stored();
	apply();
	if (started || typeof window === 'undefined' || !window.matchMedia) return;
	started = true;
	// Following the system means following it as it changes, not only at load.
	window.matchMedia(DARK).addEventListener('change', () => {
		if (theme.choice === 'system') apply();
	});
}

export function setTheme(choice) {
	theme.choice = CHOICES.includes(choice) ? choice : 'system';
	try {
		if (theme.choice === 'system') localStorage.removeItem(KEY);
		else localStorage.setItem(KEY, theme.choice);
	} catch {
		// Nothing to do: the choice still holds for this visit.
	}
	apply();
}
