AIR := $(shell command -v air 2>/dev/null || echo $(HOME)/go/bin/air)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X github.com/Infisical/agent-vault/cmd.version=$(VERSION) \
	-X github.com/Infisical/agent-vault/cmd.commit=$(COMMIT) \
	-X github.com/Infisical/agent-vault/cmd.date=$(DATE)

.PHONY: build dev test clean docker web web-dev

web:
	cd web && npm ci && npm run build

web-dev:
	cd web && npm run dev

build: web
	go build -trimpath -ldflags '$(LDFLAGS)' -o agent-vault .

# Hot-reload dev: Go backend (air) + React frontend (Vite HMR)
# Open http://localhost:5173/app/ in browser
dev: web
	@echo ""
	@echo "  ➜ Open http://localhost:5173 in your browser (not 14321)"
	@echo ""
	@trap 'kill 0' EXIT; $(AIR) & (cd web && npm run dev) & wait

test:
	go test ./...

clean:
	rm -f agent-vault
	rm -rf internal/server/webdist

docker:
	docker build -t infisical/agent-vault .
