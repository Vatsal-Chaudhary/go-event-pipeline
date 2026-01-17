APP_NAME=query-service
MAIN_PATH=cmd/main.go

.PHONY: run test build

run:
	@echo "Starting query service..."
	go run $(MAIN_PATH)

test:
	go test ./... -v

build:
	go build -o bin/$(APP_NAME) $(MAIN_PATH)
