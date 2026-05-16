DB_DSN ?= postgres://parkir:parkir@localhost:5432/parkir_pintar?sslmode=disable
MIGRATIONS_DIR := db/migrations

.PHONY: migrate-up migrate-down migrate-status db-shell

## migrate-up: apply all pending migrations in order
migrate-up:
	@echo "Applying migrations..."
	@for f in $(shell ls $(MIGRATIONS_DIR)/*.up.sql | sort); do \
		echo "  → $$f"; \
		psql "$(DB_DSN)" -f $$f; \
	done
	@echo "Done."

## migrate-down: rollback all migrations in reverse order
migrate-down:
	@echo "Rolling back migrations..."
	@for f in $(shell ls $(MIGRATIONS_DIR)/*.down.sql | sort -r); do \
		echo "  ← $$f"; \
		psql "$(DB_DSN)" -f $$f; \
	done
	@echo "Done."

## migrate-docker: apply migrations directly into the running docker postgres container
migrate-docker:
	@echo "Applying migrations to docker postgres..."
	@for f in $(shell ls $(MIGRATIONS_DIR)/*.up.sql | sort); do \
		echo "  → $$f"; \
		docker exec -i parkir_postgres psql -U parkir -d parkir_pintar < $$f; \
	done
	@echo "Done."

## db-shell: open a psql shell to the docker postgres container
db-shell:
	docker exec -it parkir_postgres psql -U parkir -d parkir_pintar
