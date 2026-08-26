# Run these on the server, from /srv/calisthenics.
#
# Deploys normally happen through GitHub Actions and nothing here is needed.
# These are the manual equivalents, for when CI is unavailable or you are
# already logged in and want to look at something.

SHELL := /bin/bash
COMPOSE := docker compose
BUILD := $(COMPOSE) -f compose.yaml -f compose.build.yaml
LIVE = $$(sed -n 's/^IMAGE_TAG=//p' .env | tail -n1)

.PHONY: help deploy deploy-tag rollback redeploy build-deploy up down restart rebuild \
        logs ps health version history migrate-status psql backup prune

help: ## Show this list
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[1m%-16s\033[0m %s\n", $$1, $$2}'

deploy: ## Pull the newest images and restart (health-checked, rolls back on failure)
	git fetch --all --prune && git reset --hard origin/main
	bash infra/deploy.sh latest

deploy-tag: ## Deploy one specific tag: make deploy-tag TAG=abc123def456
	@test -n "$(TAG)" || { echo "usage: make deploy-tag TAG=<commit-sha>"; exit 1; }
	git fetch --all --prune && git reset --hard origin/main
	bash infra/deploy.sh "$(TAG)"

redeploy: ## Re-pull and recreate whatever tag is currently live
	bash infra/deploy.sh $(LIVE)

rollback: ## Go back to the previously deployed tag
	bash infra/deploy.sh --rollback

build-deploy: ## Emergency: build the images here instead of pulling them
	git fetch --all --prune && git reset --hard origin/main
	$(BUILD) build
	$(BUILD) up -d --remove-orphans
	$(COMPOSE) ps

up:
	$(COMPOSE) up -d

down:
	$(COMPOSE) down

restart: ## Restart api and web without changing the image
	$(COMPOSE) restart api web

rebuild:
	$(COMPOSE) up -d --force-recreate

logs: ## Follow logs from every container
	$(COMPOSE) logs -f --tail=100

ps: ## Container status and health
	$(COMPOSE) ps

health: ## Ask the API whether it is actually serving
	@$(COMPOSE) exec -T api wget -qO- http://127.0.0.1:8080/healthz && echo
	@source .env && curl -sS -o /dev/null -w "public /healthz -> %{http_code}\n" "https://$$APP_DOMAIN/healthz"

version: ## Which release is live
	@echo "tag:    $(LIVE)"
	@echo "commit: $$(git rev-parse --short HEAD)"

history: ## Recent deploys
	@tail -n 20 .deploy-history 2>/dev/null || echo "no deploys recorded yet"

migrate-status:
	$(COMPOSE) exec db psql -U $${POSTGRES_USER:-cali} -d $${POSTGRES_DB:-cali} \
		-c "select version, applied_at from schema_migrations order by version;"

psql:
	$(COMPOSE) exec db psql -U $${POSTGRES_USER:-cali} -d $${POSTGRES_DB:-cali}

backup: ## Dump the database into ./backups
	@mkdir -p backups
	$(COMPOSE) exec -T db pg_dump -U $${POSTGRES_USER:-cali} $${POSTGRES_DB:-cali} \
		| gzip > backups/cali-$$(date +%Y%m%d-%H%M%S).sql.gz
	@echo "written to backups/"

prune: ## Reclaim disk from old images
	docker image prune -af
	docker builder prune -af || true
	@df -h / | tail -1
