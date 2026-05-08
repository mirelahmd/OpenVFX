.PHONY: build install-local release-build test smoke release-smoke external-install-smoke clean-local-artifacts

VERSION    ?= v0.1.0-alpha
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
MODULE     := github.com/mirelahmd/byom-video
LDFLAGS    := -X '$(MODULE)/internal/commands.Version=$(VERSION)' \
              -X '$(MODULE)/internal/commands.Commit=$(COMMIT)' \
              -X '$(MODULE)/internal/commands.BuildDate=$(BUILD_DATE)'

build:
	go build -ldflags "$(LDFLAGS)" -o byom-video ./cmd/byom-video

install-local:
	go install -ldflags "$(LDFLAGS)" ./cmd/byom-video

release-build:
	go build -ldflags "$(LDFLAGS)" -trimpath -o byom-video ./cmd/byom-video

test:
	go test ./...

smoke: build
	scripts/smoke-runs.sh

release-smoke: build
	scripts/release-smoke.sh

external-install-smoke:
	scripts/smoke-external-install.sh

clean-local-artifacts:
	@echo "Dry-run only. Review these local artifact paths before removing anything manually:"
	@echo ".byom-video/"
	@echo "exports/"
