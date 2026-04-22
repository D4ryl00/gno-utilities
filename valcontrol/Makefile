BINARY    := valcontrol
BUILD_DIR := build
CMD       := ./cmd/valcontrol

.PHONY: all build install clean test vet

all: build

build:
	go build -o $(BUILD_DIR)/$(BINARY) $(CMD)

install:
	go install $(CMD)

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f $(BUILD_DIR)/$(BINARY)
