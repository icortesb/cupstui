BIN := cupstui
PKG := ./cmd/cupstui

# CGO is off and the symbol table stripped: the result is a single static
# binary that runs on any Linux with no library to match.
BUILDFLAGS := CGO_ENABLED=0
LDFLAGS := -s -w

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

# Records docs/demo.gif. Needs vhs (which pulls in ttyd and ffmpeg) and a
# running CUPS: the tape queues held jobs, drives the interface and clears the
# queue again.
demo: build
	PATH="$(CURDIR):$$PATH" vhs docs/demo.tape

clean:
	rm -f $(BIN)
