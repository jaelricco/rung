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

// AI endpoints answer either with one JSON body or, when asked for an event
// stream, with progress events followed by the same JSON payload. stream()
// takes the second route so a call that runs for a minute can say how far
// along it is. onProgress receives {stage, label, percent, detail, done, total}.
async function stream(path, body, onProgress) {
	const response = await fetch(`/api/v1${path}`, {
		method: 'POST',
		credentials: 'same-origin',
		headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream' },
		body: JSON.stringify(body ?? {})
	});

	// Validation and auth failures answer before the stream starts, as JSON.
	if (!response.ok || !response.headers.get('content-type')?.includes('text/event-stream')) {
		let message = `Request failed (${response.status}).`;
		try {
			message = (await response.json())?.error ?? message;
		} catch {
			/* keep the status message */
		}
		const error = new Error(message);
		error.status = response.status;
		throw error;
	}

	const reader = response.body.getReader();
	const decoder = new TextDecoder();
	let buffer = '';
	let result = null;
	let failure = null;

	// Frames are separated by a blank line; anything short of one is a
	// partial read and waits for the next chunk.
	const consume = (frame) => {
		let event = 'message';
		const data = [];
		for (const line of frame.split('\n')) {
			if (line.startsWith('event:')) event = line.slice(6).trim();
			else if (line.startsWith('data:')) data.push(line.slice(5).trim());
		}
		if (data.length === 0) return;
		let payload;
		try {
			payload = JSON.parse(data.join('\n'));
		} catch {
			return;
		}
		if (event === 'progress') onProgress?.(payload);
		else if (event === 'done') result = payload;
		else if (event === 'error') failure = payload?.error ?? 'The request failed.';
	};

	for (;;) {
		const { done, value } = await reader.read();
		if (value) buffer += decoder.decode(value, { stream: true });
		let split;
		while ((split = buffer.indexOf('\n\n')) >= 0) {
			consume(buffer.slice(0, split));
			buffer = buffer.slice(split + 2);
		}
		if (done) break;
	}
	if (buffer.trim()) consume(buffer);

	if (failure) throw new Error(failure);
	if (result === null) throw new Error('The connection closed before the answer arrived.');
	return result;
}

export const api = {
	get: (path) => request('GET', path),
	post: (path, body) => request('POST', path, body ?? {}),
	put: (path, body) => request('PUT', path, body ?? {}),
	patch: (path, body) => request('PATCH', path, body),
	del: (path) => request('DELETE', path),
	stream
};
