# MacPanel: satu pintu untuk gerbang kualitas, build, dan deploy.
#   make check    gerbang penuh (format, vet, tes Go -race, UI, CLI)  ~5 menit
#   make quick    gerbang cepat tanpa -race
#   make build    UI + biner panel
#   make deploy   build lalu ganti proses panel di launchd (macOS)
#   make hooks    pasang pre-commit
SHELL := /bin/bash
MISE  := $(shell command -v mise 2>/dev/null)
GO    := $(if $(MISE),mise exec -- go,go)
PNPM  := $(if $(MISE),mise exec -- pnpm,pnpm)
PY    := python3
PKG   := .
PYFILES := cli/lab cli/lab-dev cli/lab-sys cli/lab-svc cli/lab-docker cli/panelctl

.PHONY: check quick fmt fmt-check vet lint test race ui-check ui-build cli-check build deploy hooks secrets clean

check: fmt-check vet lint race ui-check cli-check secrets
	@echo "== make check: OK =="

quick: fmt-check vet test ui-check cli-check secrets
	@echo "== make quick: OK =="

fmt:
	$(GO) fmt ./...

fmt-check:
	@out="$$($(GO) fmt ./... 2>&1 | head -5)"; \
	if [ -n "$$out" ]; then echo "gofmt mengubah berkas (jalankan make fmt):"; echo "$$out"; exit 1; fi

vet:
	$(GO) vet ./...

lint:
	@if command -v golangci-lint >/dev/null; then golangci-lint run ./...; else echo "golangci-lint tidak terpasang (brew install golangci-lint), dilewati"; fi

test:
	$(GO) test -count=1 ./...

race:
	$(GO) test -race -count=1 ./...

ui-check:
	cd ui && $(PNPM) exec tsc --noEmit && $(PNPM) run test

ui-build:
	cd ui && $(PNPM) run build

cli-check:
	@for f in $(PYFILES); do $(PY) -m py_compile $$f || exit 1; done; rm -rf cli/__pycache__
	@if command -v ruff >/dev/null; then ruff check $(PYFILES); else echo "ruff tidak terpasang (uv tool install ruff), dilewati"; fi
	@bash -n cli/addsite.sh cli/backup.sh cli/logrotate.sh install.sh scripts/*.sh

secrets:
	@bash scripts/check-secrets.sh

build: ui-build
	$(GO) build -o panel $(PKG)

deploy: build
	@bash scripts/deploy.sh

hooks:
	pre-commit install

clean:
	rm -rf ui/dist panel panel.new cli/__pycache__
