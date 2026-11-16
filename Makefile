# Atalhos simples. No Windows (sem make), use os comandos diretos do README.
.PHONY: up down logs test test-integration

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f api

# Só os testes que não precisam de banco (a integração aparece como SKIP).
test:
	MONGODB_URI= go test ./...

# Integração contra o Mongo do compose (precisa de `make up` ou `docker compose up -d mongo`).
test-integration:
	MONGODB_URI=mongodb://localhost:27017 DATABASE_NAME=kanban_management TEST_DATABASE_NAME=kanban_management_test JWT_SECRET=troque-por-um-segredo-de-pelo-menos-32-chars go test ./internal/tests/... -v
