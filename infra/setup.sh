#!/usr/bin/env bash
# The one place server setup is defined. Both entry points call this:
#
#   infra/bootstrap.sh   for a server that already exists
#   infra/cloud-init.yaml  for a server being created
#
# Assumes by the time it runs: the deploy user exists, this repo is cloned at
# /srv/calisthenics, and /root/setup.env holds APP_DOMAIN and ACME_EMAIL.
#
# Idempotent. Safe to rerun after fixing something.

set -euo pipefail
exec > >(tee -a /var/log/calisthenics-setup.log) 2>&1
echo "=== setup started $(date -Is) ==="

APP_DIR=/srv/calisthenics
DEPLOY_USER=deploy

[[ $EUID -eq 0 ]] || { echo "Run as root." >&2; exit 1; }
[[ -f /root/setup.env ]] || { echo "/root/setup.env is missing." >&2; exit 1; }
# shellcheck disable=SC1091
source /root/setup.env

: "${APP_DOMAIN:?APP_DOMAIN is not set in /root/setup.env}"
: "${ACME_EMAIL:?ACME_EMAIL is not set in /root/setup.env}"

echo "--- packages"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq ca-certificates curl git ufw fail2ban make unattended-upgrades jq

echo "--- swap"
# Deploys pull prebuilt images now, so nothing large compiles here. The swap
# stays as insurance for the fallback path in this script, which does build
# locally when the registry is unreachable.
if [[ ! -f /swapfile ]]; then
	fallocate -l 2G /swapfile
	chmod 600 /swapfile
	mkswap /swapfile
	swapon /swapfile
	echo '/swapfile none swap sw 0 0' >>/etc/fstab
	echo 'vm.swappiness=10' >/etc/sysctl.d/99-swappiness.conf
	sysctl -w vm.swappiness=10 >/dev/null
fi

echo "--- docker"
if ! command -v docker >/dev/null 2>&1; then
	install -m 0755 -d /etc/apt/keyrings
	curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
	chmod a+r /etc/apt/keyrings/docker.asc
	echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] \
https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
		>/etc/apt/sources.list.d/docker.list
	apt-get update -qq
	apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
fi
systemctl enable --now docker
usermod -aG docker "$DEPLOY_USER"

echo "--- firewall"
ufw --force reset >/dev/null
ufw default deny incoming
ufw default allow outgoing
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable
systemctl enable --now fail2ban

echo "--- ssh hardening"
cat >/etc/ssh/sshd_config.d/99-hardening.conf <<'SSHCONF'
PasswordAuthentication no
PermitRootLogin prohibit-password
KbdInteractiveAuthentication no
X11Forwarding no
SSHCONF
systemctl restart ssh 2>/dev/null || systemctl restart sshd

echo "--- automatic security updates"
cat >/etc/apt/apt.conf.d/20auto-upgrades <<'AUTOCONF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
AUTOCONF

echo "--- environment"
chown -R "$DEPLOY_USER:$DEPLOY_USER" "$APP_DIR"
if [[ ! -f "$APP_DIR/.env" ]]; then
	DB_PASSWORD="$(openssl rand -base64 36 | tr -dc 'A-Za-z0-9' | head -c 32)"
	# Seals each athlete's own provider key. The server has no model key of
	# its own to write here.
	AI_CREDENTIALS_KEY="$(openssl rand -base64 32)"
	cat >"$APP_DIR/.env" <<ENVFILE
APP_DOMAIN=$APP_DOMAIN
ACME_EMAIL=$ACME_EMAIL
POSTGRES_USER=cali
POSTGRES_PASSWORD=$DB_PASSWORD
POSTGRES_DB=cali
AI_CREDENTIALS_KEY=$AI_CREDENTIALS_KEY
AI_THINKING=adaptive
WEB_SEARCH_TOOL_VERSION=web_search_20250305
IMAGE_TAG=latest
ENVFILE
	chown "$DEPLOY_USER:$DEPLOY_USER" "$APP_DIR/.env"
	chmod 600 "$APP_DIR/.env"
	echo "    wrote $APP_DIR/.env with a generated database password and AI sealing key"
else
	echo "    $APP_DIR/.env already exists, leaving it alone"
fi

echo "--- nightly database backup"
install -d -o "$DEPLOY_USER" -g "$DEPLOY_USER" "$APP_DIR/backups"
cat >/etc/cron.d/calisthenics-backup <<'CRON'
30 3 * * * deploy cd /srv/calisthenics && /usr/bin/make backup >/dev/null 2>&1
35 3 * * * deploy find /srv/calisthenics/backups -name '*.sql.gz' -mtime +14 -delete
CRON
chmod 644 /etc/cron.d/calisthenics-backup

echo "--- start"
cd "$APP_DIR"
# Images are built in CI and pulled from GHCR. Building here is only the
# fallback for a first boot before any CI run has published them, or for a
# private package this server has no credential for.
if sudo -u "$DEPLOY_USER" docker compose pull --quiet api web 2>/dev/null; then
	sudo -u "$DEPLOY_USER" docker compose up -d --remove-orphans
else
	echo "    could not pull the published images; building them here instead"
	sudo -u "$DEPLOY_USER" docker compose -f compose.yaml -f compose.build.yaml \
		up -d --build --remove-orphans
fi

IPV4="$(curl -fsS --max-time 5 https://ipv4.icanhazip.com 2>/dev/null || hostname -I | awk '{print $1}')"
cat >/etc/motd <<MOTD

  Calisthenics training app
  ─────────────────────────
  Directory   $APP_DIR
  Status      make ps
  Logs        make logs
  Live tag    make version
  Deploy      pushes to main deploy themselves; make deploy to force one
  Rollback    make rollback
  Setup log   /var/log/calisthenics-setup.log

  Point $APP_DOMAIN at $IPV4 if you haven't.
  Caddy keeps retrying the certificate until DNS resolves.

  Coaching runs on each athlete's own Anthropic or OpenAI key, connected in
  the app under Settings. This server pays for nothing and needs no key of
  its own; AI_CREDENTIALS_KEY in $APP_DIR/.env, generated above, is only what
  seals theirs.

MOTD

echo "=== setup finished $(date -Is) ==="
echo
docker compose ps
