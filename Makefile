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

.PHONY: build-e2e test-e2e

## build-e2e: build all service Docker images required for E2E tests
build-e2e:
	@echo "Building service images..."
	docker build --network=host -t parkir-pintar/reservation:latest ./reservation
	docker build --network=host -t parkir-pintar/billing:latest ./billing
	docker build --network=host -t parkir-pintar/payment:latest ./payment
	docker build --network=host -t parkir-pintar/presence:latest ./presence
	docker build --network=host -t parkir-pintar/search:latest ./search
	docker build --network=host -t parkir-pintar/payment-gateway:latest ./stubs/payment-gateway
	docker build --network=host -t parkir-pintar/notification:latest ./stubs/notification
	@echo "Done."

## test-e2e: run E2E tests
test-e2e:
	@echo "Running E2E tests..."
	cd e2e && go test -tags=e2e -timeout 300s -v ./...
