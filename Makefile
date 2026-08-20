BIN := cupstui

.PHONY: build test run vet fmt clean

build:
	go build -o $(BIN) ./cmd/cupstui

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -s -w .

run: build
	./$(BIN)

clean:
	rm -f $(BIN)
