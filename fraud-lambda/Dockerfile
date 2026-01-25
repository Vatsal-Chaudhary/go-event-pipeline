# Build stage
FROM golang:1.25.4-alpine AS builder

WORKDIR /build

# Copy go.mod and go.sum
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build the Lambda function
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bootstrap main.go

# Runtime stage with AWS Lambda RIE
FROM public.ecr.aws/lambda/provided:al2023

# Copy the Lambda function binary
COPY --from=builder /build/bootstrap ${LAMBDA_RUNTIME_DIR}/bootstrap

# Add AWS Lambda Runtime Interface Emulator for local testing
ADD https://github.com/aws/aws-lambda-runtime-interface-emulator/releases/latest/download/aws-lambda-rie /usr/local/bin/aws-lambda-rie
RUN chmod +x /usr/local/bin/aws-lambda-rie

# Set the entrypoint
ENTRYPOINT ["/usr/local/bin/aws-lambda-rie", "/var/runtime/bootstrap"]
