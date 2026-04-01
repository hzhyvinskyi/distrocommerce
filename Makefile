.PHONY: help up down ps logs

SVC ?= grafana

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}'

# Infrastructure (Docker Compose)

up: ## Start local infrastructure (Postgres, MongoDB, Redis, Kafka, Jaeger, Prometheus, Grafana, Mailpit)
	docker compose up -d
	@echo ""
	@echo "  Kafka UI  → http://localhost:8091"
	@echo "  Grafana   → http://localhost:3000  (admin/admin)"
	@echo "  Jaeger    → http://localhost:16686"
	@echo "  Prometheus→ http://localhost:9090"
	@echo "  Mailpit   → http://localhost:8025"

down: ## Stop all containers and remove volumes
	docker compose down -v

ps: ## Show running containers
	docker compose ps

logs: ## Tail logs: make logs SVC=kafka
	docker compose logs -f $(SVC)
