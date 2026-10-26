import { sveltekit } from '@sveltejs/kit/vite';
import adapter from '@sveltejs/adapter-node';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

// SvelteKit 3 reads its configuration here rather than from a
// svelte.config.js, which it now refuses to start beside. The options are the
// same ones, unwrapped: what used to sit under `kit` is top level, and
// anything SvelteKit does not own — `preprocess` — is handed to
// vite-plugin-svelte for it.
export default {
	plugins: [
		sveltekit({
			adapter: adapter(),
			preprocess: vitePreprocess()
		})
	],
	server: {
		proxy: { '/api': 'http://localhost:8080' }
	}
};
