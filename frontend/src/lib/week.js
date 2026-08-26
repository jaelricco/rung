// A training week reads as a week, not as a list. Both the plan preview and
// the calendar lay one out the same way: four rows of two days, Monday first,
// with rest days left visibly empty so the shape of the week — which days are
// hard, where the gaps fall — is the first thing you see.

export const DAY_SHORT = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];

// 1 for Monday through 7 for Sunday, which is what the API uses.
export function isoDay(date) {
	return ((date.getDay() + 6) % 7) + 1;
}

export function mondayOf(date) {
	const monday = new Date(date);
	monday.setHours(0, 0, 0, 0);
	monday.setDate(monday.getDate() - (isoDay(monday) - 1));
	return monday;
}

export function addDays(date, days) {
	const out = new Date(date);
	out.setDate(out.getDate() + days);
	return out;
}

export function isoDate(date) {
	// Local date, not UTC: toISOString would shift the day for anyone east or
	// west of Greenwich, which puts sessions on the wrong square.
	const pad = (n) => String(n).padStart(2, '0');
	return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

// weekSlots turns whatever sits in a week into the seven squares of the grid.
// `dayOf` says which day (1-7) an item belongs to; days with nothing in them
// come back as empty slots rather than being skipped.
export function weekSlots(items, dayOf) {
	const slots = DAY_SHORT.map((short, i) => ({ day: i + 1, short, items: [] }));
	for (const item of items) {
		const day = dayOf(item);
		if (day >= 1 && day <= 7) slots[day - 1].items.push(item);
	}
	return slots;
}

// How a session is described in one line, from the blocks it contains.
export function sessionShape(session) {
	const blocks = session?.blocks ?? [];
	if (blocks.length === 0) return '';
	const sets = blocks.reduce((total, b) => total + (Number(b.sets) || 0), 0);
	return `${blocks.length} ${blocks.length === 1 ? 'block' : 'blocks'} · ${sets} sets`;
}

export function formatDate(iso) {
	const date = new Date(`${iso}T00:00:00`);
	return date.toLocaleDateString(undefined, { day: 'numeric', month: 'short' });
}
