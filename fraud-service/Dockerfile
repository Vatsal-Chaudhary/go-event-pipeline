FROM golang:1.25.4-alpine AS builder

WORKDIR /build

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o fraud-service main.go

FROM alpine:3.21

WORKDIR /app
COPY --from=builder /build/fraud-service /app/fraud-service

EXPOSE 8080

ENTRYPOINT ["/app/fraud-service"]
