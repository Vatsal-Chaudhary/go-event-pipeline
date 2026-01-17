PP_NAME=collector-service
MAIN_PATH=cmd/main.go

.PHONY: run test

run:
	@echo "🚀 Starting worker service..."
	go run $(MAIN_PATH)

test:
	go test ./... -v
