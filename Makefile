.PHONY: build run test clean deps

BINARY := rpg-helper-bot
GO_CMD := go build -o $(BINARY) ./cmd/rpg-helper-bot

build: web/dist/index.html web-player/dist/index.html $(BINARY)

$(BINARY):
	$(GO_CMD)

web/dist/index.html: web/package.json web/package-lock.json $(shell find web/src web/index.html web/vite.config.ts web/tsconfig.json -type f 2>/dev/null)
	cd web && npm install && npm run build

web-player/dist/index.html: web-player/package.json web-player/package-lock.json $(shell find web-player/src web-player/index.html web-player/vite.config.ts web-player/tsconfig.json -type f 2>/dev/null)
	cd web-player && npm install && npm run build

deps:
	cd web && npm install
	cd web-player && npm install

run: build
	./$(BINARY)

test:
	go test ./...

clean:
	rm -f $(BINARY)
	rm -rf web/dist web-player/dist
