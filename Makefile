# voila-go – build and run
BINARY_NAME := voila
MAIN_PKG := ./cmd/voila

.PHONY: build run clean test tidy proto swagger

# proto: generate Go from wire_frames.proto (requires protoc and protoc-gen-go)
proto:
	protoc --go_out=. --go_opt=paths=source_relative pkg/frames/proto/wire/wire_frames.proto

build:
	go build -o $(BINARY_NAME) $(MAIN_PKG)

run:
	go run $(MAIN_PKG)

clean:
	-go clean
	-rm -f $(BINARY_NAME) $(BINARY_NAME).exe

test:
	go test ./...

tidy:
	go mod tidy

# swagger: regenerate API docs (requires: go install github.com/swaggo/swag/cmd/swag@latest)
swagger:
	swag init -g cmd/voila/main.go --parseDependency --parseInternal
