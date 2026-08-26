#!/usr/bin/env bash
# Set up a Hetzner server that already exists.
#
# From your machine:
#   scp infra/bootstrap.sh root@YOUR-IP:/root/
#   ssh root@YOUR-IP
#   bash /root/bootstrap.sh \
#       https://github.com/YOU/YOUR-REPO.git \
#       training.example.com \
#       you@example.com
#
# Creates the deploy user, clones the repo, then hands over to infra/setup.sh
# in the repo, which is where the actual setup lives. Safe to rerun.

set -euo pipefail

REPO_URL="${1:-}"
APP_DOMAIN="${2:-}"
ACME_EMAIL="${3:-}"
APP_DIR=/srv/calisthenics
DEPLOY_USER=deploy

if [[ $EUID -ne 0 ]]; then
	echo "Run this as root." >&2
	exit 1
fi

if [[ -z "$REPO_URL" || -z "$APP_DOMAIN" || -z "$ACME_EMAIL" ]]; then
	cat >&2 <<USAGE
Usage: bash bootstrap.sh <repo-url> <app-domain> <acme-email>

  repo-url     https://github.com/you/calisthenics.git
               For a private repo, embed a read-only token:
               https://x-access-token:TOKEN@github.com/you/calisthenics.git
  app-domain   the domain whose A record points at this server
  acme-email   where Let's Encrypt sends expiry warnings
USAGE
	exit 1
fi

echo "==> Installing git"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq git ca-certificates curl

echo "==> Creating the ${DEPLOY_USER} user"
if ! id -u "$DEPLOY_USER" >/dev/null 2>&1; then
	adduser --disabled-password --gecos "" "$DEPLOY_USER"
fi
usermod -aG sudo "$DEPLOY_USER"
echo "${DEPLOY_USER} ALL=(ALL) NOPASSWD:ALL" >"/etc/sudoers.d/90-${DEPLOY_USER}"
chmod 440 "/etc/sudoers.d/90-${DEPLOY_USER}"

# Carry root's authorized keys across so you can still log in after root login
# is restricted.
if [[ -f /root/.ssh/authorized_keys ]]; then
	install -d -m 700 -o "$DEPLOY_USER" -g "$DEPLOY_USER" "/home/${DEPLOY_USER}/.ssh"
	install -m 600 -o "$DEPLOY_USER" -g "$DEPLOY_USER" \
		/root/.ssh/authorized_keys "/home/${DEPLOY_USER}/.ssh/authorized_keys"
else
	echo "!! No key found at /root/.ssh/authorized_keys." >&2
	echo "!! Add one to /home/${DEPLOY_USER}/.ssh/authorized_keys before you log out," >&2
	echo "!! or this script will lock you out when it disables password login." >&2
fi

echo "==> Cloning ${REPO_URL}"
mkdir -p "$APP_DIR"
chown "$DEPLOY_USER:$DEPLOY_USER" "$APP_DIR"
if [[ -d "$APP_DIR/.git" ]]; then
	echo "    repository already present, pulling"
	sudo -u "$DEPLOY_USER" git -C "$APP_DIR" pull --ff-only || true
else
	sudo -u "$DEPLOY_USER" git clone "$REPO_URL" "$APP_DIR"
fi

# Strip any token out of the stored remote so it isn't left in .git/config.
CLEAN_URL="$(sed -E 's#https://[^@/]+@#https://#' <<<"$REPO_URL")"
sudo -u "$DEPLOY_USER" git -C "$APP_DIR" remote set-url origin "$CLEAN_URL"

echo "==> Writing /root/setup.env"
cat >/root/setup.env <<ENVFILE
APP_DOMAIN="$APP_DOMAIN"
ACME_EMAIL="$ACME_EMAIL"
ENVFILE
chmod 600 /root/setup.env

echo "==> Handing over to infra/setup.sh"
exec bash "$APP_DIR/infra/setup.sh"
