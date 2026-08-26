# Run these on the server, from /srv/calisthenics.

.PHONY: deploy up down logs ps restart migrate-status psql backup rebuild

deploy: ## Pull the latest code and rebuild
	git fetch --all
	git reset --hard origin/main
	docker compose up -d --build --remove-orphans
	docker image prune -f
	@echo "deployed:" && docker compose ps

up:
	docker compose up -d

down:
	docker compose down

restart:
	docker compose restart api web

rebuild:
	docker compose up -d --build --force-recreate

logs:
	docker compose logs -f --tail=100

ps:
	docker compose ps

migrate-status:
	docker compose exec db psql -U $${POSTGRES_USER:-cali} -d $${POSTGRES_DB:-cali} \
		-c "select version, applied_at from schema_migrations order by version;"

psql:
	docker compose exec db psql -U $${POSTGRES_USER:-cali} -d $${POSTGRES_DB:-cali}

backup: ## Dump the database into ./backups
	@mkdir -p backups
	docker compose exec -T db pg_dump -U $${POSTGRES_USER:-cali} $${POSTGRES_DB:-cali} \
		| gzip > backups/cali-$$(date +%Y%m%d-%H%M%S).sql.gz
	@echo "written to backups/"
