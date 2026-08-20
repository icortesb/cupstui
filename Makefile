BIN := cupstui
PKG := ./cmd/cupstui

# What a local build calls itself, so -version says something truer than "dev".
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# CGO is off and the symbol table stripped: the result is a single static
# binary that runs on any Linux with no library to match.
BUILDFLAGS := CGO_ENABLED=0
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test vet fmt run install demo clean

build:
	$(BUILDFLAGS) go build -ldflags="$(LDFLAGS)" -o $(BIN) $(PKG)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -s -w .

run: build
	./$(BIN)

install:
	$(BUILDFLAGS) go install -ldflags="$(LDFLAGS)" $(PKG)

# Records docs/demo.gif. Needs vhs (which pulls in ttyd and ffmpeg), socat and a
# running CUPS. The trap goes in before the fixture so the printers and the
# configuration file are put back even if vhs fails halfway through.
demo: build
	@set -e; \
	trap './scripts/demo-fixture.sh teardown' EXIT; \
	./scripts/demo-fixture.sh setup; \
	PATH="$(CURDIR):$$PATH" vhs docs/demo.tape; \
	if command -v gifsicle >/dev/null 2>&1; then \
		gifsicle -O3 --lossy=60 docs/demo.gif -o docs/demo.gif.tmp && \
			mv docs/demo.gif.tmp docs/demo.gif; \
		echo "optimised: $$(du -h docs/demo.gif | cut -f1)"; \
	else \
		echo "gifsicle not installed, leaving docs/demo.gif as recorded" >&2; \
	fi

clean:
	rm -f $(BIN)
