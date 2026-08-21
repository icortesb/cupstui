BIN := cupstui
PKG := ./cmd/cupstui

# What a local build calls itself, so -version says something truer than "dev".
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# CGO is off and the symbol table stripped: the result is a single static
# binary that runs on any Linux with no library to match.
BUILDFLAGS := CGO_ENABLED=0
LDFLAGS := -s -w -X main.version=$(VERSION)

# The container the demo is recorded inside of. Its name is what docs/demo.tape
# attaches to, so the two have to agree.
DEMO_IMAGE := cupstui-demo
DEMO_CONTAINER := cupstui-demo

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

# Records docs/demo.gif. Needs vhs (which pulls in ttyd and ffmpeg) and podman.
#
# The print system is a container rather than the machine's own CUPS: the GIF
# is published, and the printers, home directory and page log of whoever
# records it are nobody's business. The trap goes in before the container is
# started so it is removed even if vhs fails halfway through.
demo: build
	@set -e; \
	podman build -q -f scripts/demo.Containerfile -t $(DEMO_IMAGE) scripts >/dev/null; \
	trap 'podman rm -f $(DEMO_CONTAINER) >/dev/null 2>&1' EXIT; \
	podman rm -f $(DEMO_CONTAINER) >/dev/null 2>&1 || true; \
	podman run -d --name $(DEMO_CONTAINER) $(DEMO_IMAGE) sleep infinity >/dev/null; \
	podman cp $(BIN) $(DEMO_CONTAINER):/usr/local/bin/cupstui; \
	podman cp scripts/demo-fixture.sh $(DEMO_CONTAINER):/usr/local/bin/demo-fixture.sh; \
	podman exec $(DEMO_CONTAINER) /usr/local/bin/demo-fixture.sh setup; \
	vhs docs/demo.tape; \
	if command -v gifsicle >/dev/null 2>&1; then \
		gifsicle -O3 --lossy=60 docs/demo.gif -o docs/demo.gif.tmp && \
			mv docs/demo.gif.tmp docs/demo.gif; \
		echo "optimised: $$(du -h docs/demo.gif | cut -f1)"; \
	else \
		echo "gifsicle not installed, leaving docs/demo.gif as recorded" >&2; \
	fi

clean:
	rm -f $(BIN)
