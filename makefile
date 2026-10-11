IMAGE_NAME := popofinder
DOCKER_TAG := $(shell git rev-parse --short HEAD)
DOCKER_USER := $(shell echo $$DOCKER_USER)
PLATFORMS := linux/arm64/v8,linux/amd64
IT_CONTAINER := popofinder-it
IT_PORT := 55432
IT_DB_URL := postgres://postgres:it@127.0.0.1:$(IT_PORT)/postgres?sslmode=disable

.PHONY: builder build local-build ci test-integration

builder:
	docker buildx create --name mybuilder --use || true
	docker buildx use mybuilder

build:
	docker buildx build --platform=$(PLATFORMS) \
		--push \
		-t $(DOCKER_USER)/$(IMAGE_NAME):$(DOCKER_TAG) \
		-t $(DOCKER_USER)/$(IMAGE_NAME):latest .

local-build:
	docker buildx build --load -t $(DOCKER_USER)/$(IMAGE_NAME):local .

local-run:
	docker run -p 8000:8000 $(DOCKER_USER)/$(IMAGE_NAME):local

ci: builder build

test-integration:
	docker rm -f $(IT_CONTAINER) >/dev/null 2>&1 || true
	docker run -d --rm --name $(IT_CONTAINER) -e POSTGRES_PASSWORD=it -p 127.0.0.1:$(IT_PORT):5432 postgres:18.6-bookworm >/dev/null
	@for i in $$(seq 1 60); do docker exec $(IT_CONTAINER) pg_isready -h 127.0.0.1 -U postgres -q && break; sleep 1; done
	@set -e; for f in migrations/*.up.sql; do docker exec -i $(IT_CONTAINER) psql -q -U postgres -v ON_ERROR_STOP=1 < $$f; done
	TEST_DATABASE_URL='$(IT_DB_URL)' go test -tags integration -count=1 -v ./internal/database/; status=$$?; \
	docker exec -i $(IT_CONTAINER) psql -q -U postgres -v ON_ERROR_STOP=1 < $$(ls migrations/*.down.sql | sort | tail -n 1) || status=1; \
	docker rm -f $(IT_CONTAINER) >/dev/null; exit $$status
