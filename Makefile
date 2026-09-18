.PHONY: build test clean run example web release lint docker-build docker-push

BINARY=ingress2apisix
VERSION=v0.6.0

GO ?= go
DOCKER ?= docker
BUILDX ?= $(DOCKER) buildx
BUILDER ?= default
PLATFORMS ?= linux/amd64,linux/arm64
IMAGE ?= hub.easystack.io/captain/ingress2apisix
IMAGE_TAG ?= $(VERSION)

build:
	$(GO) build -o bin/$(BINARY) ./cmd/ingress2apisix

test:
	$(GO) test ./... -v

run: build
	./bin/$(BINARY) -f examples/ingress.yaml

web: build
	./bin/$(BINARY) --web

example:
	./bin/$(BINARY) -f examples/ingress.yaml -o examples/output.yaml

clean:
	rm -rf bin/ dist/ pkg/web/static/dist/

lint:
	$(GO) vet ./...

release: clean
	@mkdir -p dist
	GOOS=linux   GOARCH=amd64 $(GO) build -o dist/$(BINARY)-linux-amd64        ./cmd/ingress2apisix
	GOOS=darwin  GOARCH=arm64 $(GO) build -o dist/$(BINARY)-darwin-arm64       ./cmd/ingress2apisix
	GOOS=darwin  GOARCH=amd64 $(GO) build -o dist/$(BINARY)-darwin-amd64       ./cmd/ingress2apisix
	GOOS=windows GOARCH=amd64 $(GO) build -o dist/$(BINARY)-windows-amd64.exe  ./cmd/ingress2apisix
	@mkdir -p pkg/web/static/dist
	@cp dist/* pkg/web/static/dist/
	@echo "Release binaries built and embedded in pkg/web/static/dist/"
	@ls -lh pkg/web/static/dist/

docker-build:
	$(BUILDX) build --builder $(BUILDER) --platform $(PLATFORMS) --output type=oci,dest=dist/$(BINARY)-$(VERSION).tar .

docker-push:
	$(BUILDX) build --builder $(BUILDER) --platform $(PLATFORMS) -t $(IMAGE):$(IMAGE_TAG) --push .
