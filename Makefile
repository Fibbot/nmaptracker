BINARY := nmap-tracker
GOCACHE ?= $(CURDIR)/.gocache
GOMODCACHE ?= $(CURDIR)/.gomodcache
GOENV := GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE)
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: test build build-all

test:
	env -u GOROOT $(GOENV) go test ./...

build:
	env -u GOROOT $(GOENV) go build ./cmd/$(BINARY)

build-all:
	@mkdir -p dist
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		out=dist/$(BINARY)-$$os-$$arch; \
		if [ $$os = windows ]; then out=$$out.exe; fi; \
		echo "building $$out"; \
		env -u GOROOT $(GOENV) GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -o $$out ./cmd/$(BINARY); \
	done
