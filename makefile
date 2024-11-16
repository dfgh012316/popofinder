IMAGE_NAME := popofinder
DOCKER_TAG := $(shell git rev-parse --short HEAD)
DOCKER_USER := $(shell echo $$DOCKER_USER)
PLATFORMS := linux/arm64/v8,linux/amd64

.PHONY: builder build local-build ci

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