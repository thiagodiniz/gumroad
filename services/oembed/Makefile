SERVICE    := oembed
IMAGE_REPO ?= ghcr.io/thiagodiniz/$(SERVICE)
IMAGE_TAG  ?= $(shell git rev-parse --short=12 HEAD)
IMAGE      := $(IMAGE_REPO):$(IMAGE_TAG)
GO         ?= go

.PHONY: build run test lint fmt docker-build docker-run k8s-render k8s-apply

build:
	$(GO) build -o bin/server ./cmd/server

run: build
	PORT=8080 ./bin/server

test:
	$(GO) test -race -count=1 ./...

lint:
	test -z "$$(gofmt -l .)" || (gofmt -l . && echo "gofmt: files need formatting" && exit 1)
	$(GO) vet ./...

fmt:
	gofmt -w .

docker-build:
	docker build --build-arg VERSION=$(IMAGE_TAG) -t $(IMAGE) .

docker-run: docker-build
	docker run --rm -p 8080:8080 --env-file .env $(IMAGE)

# OVERLAY is a directory under deploy/k8s/overlays (staging, production).
OVERLAY ?= staging

k8s-render:
	kubectl kustomize deploy/k8s/overlays/$(OVERLAY)

k8s-apply:
	kubectl apply -k deploy/k8s/overlays/$(OVERLAY)
