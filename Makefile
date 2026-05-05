.PHONY: build test smoke release-smoke clean-local-artifacts

build:
	go build -o byom-video ./cmd/byom-video

test:
	go test ./...

smoke: build
	scripts/smoke-runs.sh

release-smoke: build
	scripts/release-smoke.sh

clean-local-artifacts:
	@echo "Dry-run only. Review these local artifact paths before removing anything manually:"
	@echo ".byom-video/"
	@echo "exports/"
