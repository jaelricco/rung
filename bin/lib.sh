#!/usr/bin/env bash
# Shared plumbing for bin/deploy and bin/ops.
#
# Everything here talks to the GitHub API over HTTPS and nothing else. That is
# deliberate: the machines these scripts run on may have no route to the server
# at all, and no docker, and no ssh. GitHub's runner has all three, so the
# runner does the work and these scripts drive it.

set -euo pipefail

# shellcheck disable=SC2034
BOLD=$'\033[1m'; DIM=$'\033[2m'; RED=$'\033[31m'; GREEN=$'\033[32m'
YELLOW=$'\033[33m'; RESET=$'\033[0m'

say()  { printf '%s==>%s %s\n' "$BOLD" "$RESET" "$*"; }
info() { printf '%s    %s%s\n' "$DIM" "$*" "$RESET"; }
ok()   { printf '%s ok %s %s\n' "$GREEN" "$RESET" "$*"; }
warn() { printf '%s  ! %s %s\n' "$YELLOW" "$RESET" "$*" >&2; }
die()  { printf '%s xx %s %s\n' "$RED" "$RESET" "$*" >&2; exit 1; }

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Credentials live outside the working tree by default, so that a careless
# `git add -A` cannot reach them. CALI_SECRETS overrides the location.
load_secrets() {
	local candidates=(
		"${CALI_SECRETS:-}"
		"$REPO_ROOT/../.secrets/deploy.env"
		"$REPO_ROOT/.secrets/deploy.env"
		"$HOME/.config/cali/deploy.env"
	)
	local f
	for f in "${candidates[@]}"; do
		[[ -n "$f" && -f "$f" ]] || continue
		set -a
		# shellcheck source=/dev/null
		source "$f"
		set +a
		export SECRETS_FILE="$f"
		return 0
	done
	return 1
}

require_token() {
	load_secrets || true
	[[ -n "${GITHUB_TOKEN:-}" ]] || die "no GITHUB_TOKEN. Put it in ../.secrets/deploy.env or export it."
}

REPO="${GITHUB_REPO:-}"
detect_repo() {
	[[ -n "$REPO" ]] && return 0
	local url
	url="$(git -C "$REPO_ROOT" remote get-url origin 2>/dev/null || true)"
	[[ -n "$url" ]] || die "cannot work out the repository: no origin remote and GITHUB_REPO is unset."
	REPO="$(sed -E 's#(git@github.com:|https://github.com/)##; s#\.git$##' <<<"$url")"
}

# gh_api METHOD PATH [json-body]
gh_api() {
	local method="$1" path="$2" body="${3:-}" url
	case "$path" in
	http*) url="$path" ;;
	*) url="https://api.github.com$path" ;;
	esac
	local args=(-sS -X "$method" -H "Authorization: Bearer $GITHUB_TOKEN"
		-H "Accept: application/vnd.github+json"
		-H "X-GitHub-Api-Version: 2022-11-28")
	[[ -n "$body" ]] && args+=(-H "Content-Type: application/json" -d "$body")
	curl "${args[@]}" "$url"
}

run_url() { printf 'https://github.com/%s/actions/runs/%s\n' "$REPO" "$1"; }

# Blocks until the given run finishes, printing each job as it settles.
watch_run() {
	local run_id="$1" seen="" status conclusion
	while :; do
		local jobs_json
		jobs_json="$(gh_api GET "/repos/$REPO/actions/runs/$run_id/jobs?per_page=50")"
		while IFS=$'\t' read -r name status conclusion; do
			[[ -n "$name" ]] || continue
			[[ "$status" != "completed" ]] && continue
			grep -qxF "$name" <<<"$seen" && continue
			seen+="$name"$'\n'
			case "$conclusion" in
			success) ok "$name" ;;
			skipped) info "$name (skipped)" ;;
			*) warn "$name: $conclusion" ;;
			esac
		done < <(printf '%s' "$jobs_json" | python3 -c '
import json,sys
for j in json.load(sys.stdin).get("jobs",[]):
    print(j["name"], j["status"], j.get("conclusion") or "", sep="\t")')

		local run_json
		run_json="$(gh_api GET "/repos/$REPO/actions/runs/$run_id")"
		read -r status conclusion < <(printf '%s' "$run_json" | python3 -c '
import json,sys
d=json.load(sys.stdin); print(d["status"], d.get("conclusion") or "-")')
		# Read by the callers once this returns.
		[[ "$status" == "completed" ]] && { export RUN_CONCLUSION="$conclusion"; return 0; }
		sleep 6
	done
}

# The ops workflow and the deploy job both post their transcript to an issue,
# because Actions log downloads come from a storage host that a locked-down
# network usually cannot reach.
ops_issue_number() {
	[[ -n "${OPS_ISSUE:-}" ]] && { printf '%s' "$OPS_ISSUE"; return 0; }
	OPS_ISSUE="$(gh_api GET "/repos/$REPO/issues?state=all&per_page=100" |
		python3 -c '
import json,sys
for i in json.load(sys.stdin):
    if i["title"].strip().lower()=="ops log" and "pull_request" not in i:
        print(i["number"]); break')"
	[[ -n "$OPS_ISSUE" ]] || return 1
	printf '%s' "$OPS_ISSUE"
}

# Prints the newest comment. With an argument, prints nothing unless a comment
# newer than that id exists — otherwise a run that has not posted yet reads as
# the previous run's output, which is worse than no output at all.
latest_ops_comment() {
	local after="${1:-0}" issue
	issue="$(ops_issue_number)" || return 1
	gh_api GET "/repos/$REPO/issues/$issue/comments?per_page=1&sort=created&direction=desc" |
		python3 -c '
import json,sys
after=int(sys.argv[1])
c=json.load(sys.stdin)
if c and c[0]["id"]>after:
    print(c[0]["body"])' "$after"
}

latest_ops_comment_id() {
	local issue
	issue="$(ops_issue_number)" || { printf '0'; return 0; }
	gh_api GET "/repos/$REPO/issues/$issue/comments?per_page=1&sort=created&direction=desc" |
		python3 -c '
import json,sys
c=json.load(sys.stdin)
print(c[0]["id"] if c else 0)'
}
