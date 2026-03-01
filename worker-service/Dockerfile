FROM golang:1.25.4-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o worker-service ./cmd/main.go

FROM alpine:3.21

WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /build/worker-service /app/worker-service
COPY --from=builder /build/internal/db/migrations /app/internal/db/migrations

ENTRYPOINT ["/app/worker-service"]
