#!/usr/bin/env bash
# Pull-based deploy, run on the server from /srv/calisthenics.
#
#   infra/deploy.sh <tag>        deploy that image tag
#   infra/deploy.sh --rollback   go back to the previously deployed tag
#
# Nothing is compiled here. The tag is a commit SHA that CI has already built
# and pushed to GHCR. The tag currently live is stored in .env, so every other
# compose command in this directory operates on the same release.
#
# The contract: this script either leaves the stack healthy on the new tag, or
# healthy on the old one. It never exits leaving a broken deploy running.

set -euo pipefail

APP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$APP_DIR"

ENV_FILE="$APP_DIR/.env"
PREV_FILE="$APP_DIR/.deploy-prev"
HISTORY="$APP_DIR/.deploy-history"
HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-180}"

log() { printf '\033[1m==> %s\033[0m\n' "$*"; }
warn() { printf '\033[33m--- %s\033[0m\n' "$*" >&2; }
die() { printf '\033[31mxxx %s\033[0m\n' "$*" >&2; exit 1; }

[[ -f "$ENV_FILE" ]] || die ".env is missing. Run infra/setup.sh first."

# Reads a key out of .env without sourcing it — .env holds a generated database
# password that may contain characters a shell would try to interpret.
env_get() {
	sed -n "s/^$1=//p" "$ENV_FILE" | tail -n1
}

env_set() {
	local key="$1" value="$2" tmp
	tmp="$(mktemp "$APP_DIR/.env.XXXXXX")"
	if grep -q "^$key=" "$ENV_FILE"; then
		sed "s|^$key=.*|$key=$value|" "$ENV_FILE" >"$tmp"
	else
		cat "$ENV_FILE" >"$tmp"
		printf '%s=%s\n' "$key" "$value" >>"$tmp"
	fi
	# Same permissions and ownership as the file being replaced.
	chmod --reference="$ENV_FILE" "$tmp"
	mv "$tmp" "$ENV_FILE"
}

# Waits until every container compose knows about is either healthy or, for the
# ones without a healthcheck, simply running. Anything that exits or reports
# unhealthy fails immediately rather than burning the whole timeout.
wait_healthy() {
	local deadline=$((SECONDS + HEALTH_TIMEOUT)) ids id name state health pending
	while ((SECONDS < deadline)); do
		pending=""
		# -a so a container that exited is still listed: without it a crashed
		# service simply disappears and this loop would call that healthy.
		ids="$(docker compose ps -aq)"
		[[ -n "$ids" ]] || { sleep 3; continue; }
		while read -r id; do
			[[ -n "$id" ]] || continue
			name="$(docker inspect -f '{{.Name}}' "$id" | sed 's|^/||')"
			state="$(docker inspect -f '{{.State.Status}}' "$id")"
			health="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$id")"
			case "$state:$health" in
			running:healthy | running:none) ;;
			running:starting) pending="$pending $name" ;;
			running:unhealthy) return 1 ;;
			*) return 1 ;;
			esac
		done <<<"$ids"
		[[ -z "$pending" ]] && return 0
		sleep 3
	done
	warn "timed out after ${HEALTH_TIMEOUT}s waiting for:$pending"
	return 1
}

# The stack can be "healthy" while the API answers 500 on every request, so ask
# it the one question that exercises the process and its database connection.
check_api() {
	docker compose exec -T api wget -qO- http://127.0.0.1:8080/healthz >/dev/null 2>&1
}

bring_up() {
	docker compose pull --quiet api web
	docker compose up -d --remove-orphans
}

CURRENT="$(env_get IMAGE_TAG)"
CURRENT="${CURRENT:-latest}"

if [[ "${1:-}" == "--rollback" ]]; then
	[[ -f "$PREV_FILE" ]] || die "no previous deploy recorded, so there is nothing to roll back to."
	TARGET="$(cat "$PREV_FILE")"
	ROLLBACK_ALLOWED=false
	log "rolling back: $CURRENT -> $TARGET"
else
	TARGET="${1:-}"
	[[ -n "$TARGET" ]] || die "usage: infra/deploy.sh <tag> | --rollback"
	ROLLBACK_ALLOWED=true
	log "deploying $TARGET (currently $CURRENT)"
fi

[[ "$TARGET" == "$CURRENT" ]] && warn "$TARGET is already the live tag; redeploying anyway"

env_set IMAGE_TAG "$TARGET"

if ! bring_up; then
	warn "pull or start failed"
	if [[ "$ROLLBACK_ALLOWED" == true && "$CURRENT" != "$TARGET" ]]; then
		log "restoring $CURRENT"
		env_set IMAGE_TAG "$CURRENT"
		bring_up || die "the rollback also failed. The stack needs a human."
		die "deploy of $TARGET failed to start; $CURRENT is live again."
	fi
	die "deploy of $TARGET failed to start."
fi

log "waiting for containers to report healthy (up to ${HEALTH_TIMEOUT}s)"
if wait_healthy && check_api; then
	# Only record the predecessor once the new tag is actually serving,
	# otherwise a second failed deploy would overwrite the last good tag with a
	# broken one and make rollback useless.
	[[ "$CURRENT" != "$TARGET" ]] && printf '%s\n' "$CURRENT" >"$PREV_FILE"
	printf '%s\t%s\n' "$(date -Is)" "$TARGET" >>"$HISTORY"
	docker image prune -f >/dev/null
	log "$TARGET is live"
	docker compose ps
	exit 0
fi

warn "$TARGET did not become healthy"
docker compose logs --tail=60 api web || true

if [[ "$ROLLBACK_ALLOWED" == true && "$CURRENT" != "$TARGET" ]]; then
	log "rolling back to $CURRENT"
	env_set IMAGE_TAG "$CURRENT"
	if bring_up && wait_healthy && check_api; then
		printf '%s\t%s\t(rollback from %s)\n' "$(date -Is)" "$CURRENT" "$TARGET" >>"$HISTORY"
		die "deploy of $TARGET failed its health check. Rolled back; $CURRENT is live."
	fi
	die "deploy of $TARGET failed AND the rollback to $CURRENT did not come up. The stack needs a human."
fi

die "deploy of $TARGET failed its health check."
