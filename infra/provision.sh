#!/usr/bin/env bash
# Provision the server from your own machine, with your own credentials.
#
# Requires the Hetzner CLI:
#   brew install hcloud            (macOS)
#   or grab a release from https://github.com/hetznercloud/cli/releases
#
# Authenticate once, interactively. The token is stored by hcloud in your
# config, never passed on a command line where it would land in shell history:
#   hcloud context create calisthenics
#
# Then:
#   bash infra/provision.sh
#
# Re-running is safe: every step checks whether the resource already exists.

set -euo pipefail

SERVER_NAME="${SERVER_NAME:-calisthenics}"
SERVER_TYPE="${SERVER_TYPE:-cx33}"     # 4 vCPU x86, 8 GB RAM, 80 GB disk
LOCATION="${LOCATION:-nbg1}"           # nbg1 Nuremberg, fsn1 Falkenstein, hel1 Helsinki
IMAGE="${IMAGE:-ubuntu-24.04}"
SSH_KEY_NAME="${SSH_KEY_NAME:-$SERVER_NAME-key}"
SSH_PUBLIC_KEY="${SSH_PUBLIC_KEY:-$HOME/.ssh/id_ed25519.pub}"
FIREWALL_NAME="${FIREWALL_NAME:-$SERVER_NAME-fw}"

command -v hcloud >/dev/null 2>&1 || {
	echo "hcloud is not installed. See https://github.com/hetznercloud/cli" >&2
	exit 1
}

if ! hcloud server list >/dev/null 2>&1; then
	echo "hcloud isn't authenticated. Run: hcloud context create $SERVER_NAME" >&2
	exit 1
fi

echo "==> Checking the server type is available in $LOCATION"
if ! hcloud server-type describe "$SERVER_TYPE" >/dev/null 2>&1; then
	echo "Server type $SERVER_TYPE not found. Current list:" >&2
	hcloud server-type list >&2
	exit 1
fi

echo "==> SSH key"
if [[ ! -f "$SSH_PUBLIC_KEY" ]]; then
	echo "No public key at $SSH_PUBLIC_KEY." >&2
	echo "Create one with: ssh-keygen -t ed25519 -C \"$SERVER_NAME\"" >&2
	exit 1
fi
if hcloud ssh-key describe "$SSH_KEY_NAME" >/dev/null 2>&1; then
	echo "    $SSH_KEY_NAME already uploaded"
else
	hcloud ssh-key create --name "$SSH_KEY_NAME" --public-key-from-file "$SSH_PUBLIC_KEY"
fi

echo "==> Firewall"
# Two layers: this one filters at Hetzner's edge, ufw on the host filters again.
if hcloud firewall describe "$FIREWALL_NAME" >/dev/null 2>&1; then
	echo "    $FIREWALL_NAME already exists"
else
	hcloud firewall create --name "$FIREWALL_NAME"
	for port in 22 80 443; do
		hcloud firewall add-rule "$FIREWALL_NAME" \
			--direction in --protocol tcp --port "$port" \
			--source-ips 0.0.0.0/0 --source-ips ::/0 \
			--description "allow tcp/$port"
	done
fi

echo "==> Server"
if hcloud server describe "$SERVER_NAME" >/dev/null 2>&1; then
	echo "    $SERVER_NAME already exists, leaving it alone"
else
	hcloud server create \
		--name "$SERVER_NAME" \
		--type "$SERVER_TYPE" \
		--image "$IMAGE" \
		--location "$LOCATION" \
		--ssh-key "$SSH_KEY_NAME" \
		--firewall "$FIREWALL_NAME" \
		--enable-backup \
		--label app=calisthenics
fi

IPV4="$(hcloud server ip "$SERVER_NAME")"

cat <<DONE

==> Server ready at $IPV4

  1. Point your domain's A record at $IPV4 and wait for it to resolve:
       dig +short your-domain.example

  2. Run the one-time setup:
       scp infra/bootstrap.sh root@$IPV4:/root/
       ssh root@$IPV4 'bash /root/bootstrap.sh https://github.com/YOU/YOUR-REPO.git'

  3. Fill in the environment and start it:
       ssh deploy@$IPV4
       nano /srv/calisthenics/.env
       cd /srv/calisthenics && make deploy

Backups are on (billed at 20% of the server price). They snapshot the whole
server, which is not the same as a database backup you can restore selectively,
so keep running \`make backup\` and pull those dumps off the box.

To tear everything down:
  hcloud server delete $SERVER_NAME
  hcloud firewall delete $FIREWALL_NAME
DONE
