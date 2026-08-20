BIN := cupstui
PKG := ./cmd/cupstui

# CGO is off and the symbol table stripped: the result is a single static
# binary that runs on any Linux with no library to match.
BUILDFLAGS := CGO_ENABLED=0
LDFLAGS := -s -w

.PHONY: build test vet fmt run install clean

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

clean:
	rm -f $(BIN)
