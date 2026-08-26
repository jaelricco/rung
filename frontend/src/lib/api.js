// Thin wrapper over fetch. The session lives in an HttpOnly cookie, so
// requests just need credentials included.
async function request(method, path, body) {
	const options = {
		method,
		credentials: 'same-origin',
		headers: {}
	};
	if (body !== undefined) {
		options.headers['Content-Type'] = 'application/json';
		options.body = JSON.stringify(body);
	}

	const response = await fetch(`/api/v1${path}`, options);
	const text = await response.text();
	let payload = null;
	if (text) {
		try {
			payload = JSON.parse(text);
		} catch {
			payload = null;
		}
	}

	if (!response.ok) {
		const message = payload?.error ?? `Request failed (${response.status}).`;
		const error = new Error(message);
		error.status = response.status;
		throw error;
	}
	return payload;
}

export const api = {
	get: (path) => request('GET', path),
	post: (path, body) => request('POST', path, body ?? {}),
	patch: (path, body) => request('PATCH', path, body),
	del: (path) => request('DELETE', path)
};
