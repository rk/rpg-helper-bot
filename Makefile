.PHONY: build run test clean deps

BINARY := rpg-helper-bot
GO_CMD := go build -o $(BINARY) ./cmd/rpg-helper-bot

WEB_SRCS := $(shell find web/src web/index.html web/vite.config.ts web/tsconfig.json -type f 2>/dev/null)
WEB_PLAYER_SRCS := $(shell find web-player/src web-player/index.html web-player/vite.config.ts web-player/tsconfig.json -type f 2>/dev/null)

build: web/dist/index.html web-player/dist/index.html $(BINARY)

$(BINARY):
	$(GO_CMD)

web/node_modules/.install-stamp: web/package.json web/package-lock.json
	cd web && sfw npm ci
	@touch $@

web-player/node_modules/.install-stamp: web-player/package.json web-player/package-lock.json
	cd web-player && sfw npm ci
	@touch $@

web/dist/index.html: web/node_modules/.install-stamp $(WEB_SRCS)
	cd web && npm run build

web-player/dist/index.html: web-player/node_modules/.install-stamp $(WEB_PLAYER_SRCS)
	cd web-player && npm run build

deps: web/node_modules/.install-stamp web-player/node_modules/.install-stamp

run: build
	./$(BINARY)

test:
	go test ./...

clean:
	rm -f $(BINARY)
	rm -rf web/dist web-player/dist
