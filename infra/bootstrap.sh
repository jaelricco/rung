#!/usr/bin/env bash
# One-time setup for a fresh Hetzner Ubuntu 24.04 server.
#
# Run as root on the new box:
#   bash bootstrap.sh git@github.com:you/calisthenics.git
#
# Afterwards: fill in /srv/calisthenics/.env, then run `make deploy` there.

set -euo pipefail

REPO_URL="${1:-}"
APP_DIR="/srv/calisthenics"
DEPLOY_USER="deploy"

if [[ $EUID -ne 0 ]]; then
	echo "Run this as root." >&2
	exit 1
fi

echo "==> Updating packages"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get upgrade -y -qq
apt-get install -y -qq ca-certificates curl git ufw fail2ban unattended-upgrades make

echo "==> Installing Docker"
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

echo "==> Creating the ${DEPLOY_USER} user"
if ! id -u "$DEPLOY_USER" >/dev/null 2>&1; then
	adduser --disabled-password --gecos "" "$DEPLOY_USER"
fi
usermod -aG docker "$DEPLOY_USER"

# Carry root's authorized keys over so you can still get in as deploy.
if [[ -f /root/.ssh/authorized_keys ]]; then
	install -d -m 700 -o "$DEPLOY_USER" -g "$DEPLOY_USER" "/home/${DEPLOY_USER}/.ssh"
	install -m 600 -o "$DEPLOY_USER" -g "$DEPLOY_USER" \
		/root/.ssh/authorized_keys "/home/${DEPLOY_USER}/.ssh/authorized_keys"
fi

echo "==> Firewall"
ufw --force reset >/dev/null
ufw default deny incoming
ufw default allow outgoing
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable

echo "==> Hardening SSH"
cat >/etc/ssh/sshd_config.d/99-hardening.conf <<'SSHCONF'
PasswordAuthentication no
PermitRootLogin prohibit-password
KbdInteractiveAuthentication no
X11Forwarding no
SSHCONF
systemctl restart ssh || systemctl restart sshd

echo "==> Automatic security updates"
cat >/etc/apt/apt.conf.d/20auto-upgrades <<'AUTOCONF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
AUTOCONF

echo "==> Preparing ${APP_DIR}"
mkdir -p "$APP_DIR"
chown "${DEPLOY_USER}:${DEPLOY_USER}" "$APP_DIR"

if [[ -n "$REPO_URL" ]]; then
	if [[ -d "${APP_DIR}/.git" ]]; then
		echo "    repository already present, leaving it alone"
	else
		sudo -u "$DEPLOY_USER" git clone "$REPO_URL" "$APP_DIR"
	fi
	if [[ ! -f "${APP_DIR}/.env" && -f "${APP_DIR}/.env.example" ]]; then
		sudo -u "$DEPLOY_USER" cp "${APP_DIR}/.env.example" "${APP_DIR}/.env"
		sudo -u "$DEPLOY_USER" sed -i \
			"s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=$(openssl rand -base64 32 | tr -d '/+=')|" \
			"${APP_DIR}/.env"
		echo "    wrote ${APP_DIR}/.env with a generated database password"
	fi
fi

cat <<'DONE'

==> Done.

Next:
  1. ssh in as deploy:            ssh deploy@<server-ip>
  2. edit the remaining values:   nano /srv/calisthenics/.env
                                  (APP_DOMAIN, ACME_EMAIL, ANTHROPIC_API_KEY)
  3. point your domain's A record at this server's IP
  4. bring it up:                 cd /srv/calisthenics && make deploy

Caddy requests the TLS certificate on first start, so DNS must resolve first.
DONE
