# MacPanel: satu pintu untuk gerbang kualitas, build, dan deploy.
#   make check    gerbang penuh (format, vet, tes Go -race, UI, CLI)  ~5 menit
#   make quick    gerbang cepat tanpa -race
#   make cli-live tes CLI yang menyentuh mesin nyata (database sementara zz_*)
#   make build    UI + biner panel
#   make deploy   build lalu ganti proses panel di launchd (macOS)
#   make hooks    pasang pre-commit
SHELL := /bin/bash
MISE  := $(shell command -v mise 2>/dev/null)
GO    := $(if $(MISE),mise exec -- go,go)
PNPM  := $(if $(MISE),mise exec -- pnpm,pnpm)
PY    := python3
PKG   := ./cmd/panel
PYSRC := cli cli/lab cli/panelctl

guide-check:
	@bash scripts/guide.sh check

usage:
	@bash scripts/guide.sh usage

.PHONY: guide-check usage check quick fmt fmt-check vet lint test race gen-api api-check ui-check ui-build cli-check cli-test cli-live build deploy hooks secrets clean

check: fmt-check vet lint race api-check ui-check cli-check guide-check secrets
	@echo "== make check: OK =="

quick: fmt-check vet test api-check ui-check cli-check secrets
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

# Spesifikasi OpenAPI dan tipe path UI diturunkan dari tabel rute Go. Edit rute lalu jalankan `make gen-api`.
gen-api:
	$(GO) run ./cmd/genapi

api-check:
	$(GO) run ./cmd/genapi -check

ui-check:
	cd ui && $(PNPM) exec tsc --noEmit && $(PNPM) run test

ui-build:
	cd ui && $(PNPM) run build

cli-check:
	@$(PY) -m compileall -q cli/labcli cli/lab cli/panelctl; find cli -name __pycache__ -prune -exec rm -rf {} + 2>/dev/null || true
	@if command -v ruff >/dev/null; then ruff check $(PYSRC) && ruff format --check $(PYSRC); else echo "ruff tidak terpasang (uv tool install ruff), dilewati"; fi
	@bash -n cli/addsite.sh cli/backup.sh cli/logrotate.sh install.sh scripts/*.sh
	@$(MAKE) --no-print-directory cli-test

cli-test:
	@if command -v uv >/dev/null; then uv run --with pytest python -m pytest cli/tests -q; \
	elif $(PY) -c 'import pytest' 2>/dev/null; then $(PY) -m pytest cli/tests -q; \
	else echo "pytest tidak tersedia (brew install uv)"; exit 1; fi; rc=$$?; find cli -name __pycache__ -prune -exec rm -rf {} + 2>/dev/null || true; exit $$rc

cli-live:
	MACPANEL_LIVE=1 uv run --with pytest python -m pytest cli/tests -q -m live

secrets:
	@bash scripts/check-secrets.sh

build: ui-build
	$(GO) build -o panel $(PKG)

deploy: build
	@bash scripts/guide.sh install
	@bash scripts/deploy.sh

hooks:
	pre-commit install

clean:
	rm -rf ui/dist panel panel.new cli/__pycache__
