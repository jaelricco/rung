import { api } from './api.js';

// One shared piece of state: who is signed in. `null` means signed out,
// `undefined` means we haven't checked yet.
export const session = $state({ user: undefined });

export async function loadUser() {
	try {
		session.user = await api.get('/me');
	} catch {
		session.user = null;
	}
	return session.user;
}

export async function signOut() {
	try {
		await api.post('/auth/logout');
	} finally {
		session.user = null;
	}
}
