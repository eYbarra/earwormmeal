# Build stage — use latest Go to match go.mod toolchain requirement
FROM golang:latest AS builder

WORKDIR /app

# Copy dependency files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with CGO disabled (modernc.org/sqlite is pure Go)
RUN CGO_ENABLED=0 GOOS=linux go build -o earworm ./cmd/server

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from build stage
COPY --from=builder /app/earworm .

# Copy static frontend assets
COPY --from=builder /app/web/ ./web/

EXPOSE 8080

ENTRYPOINT ["./earworm"]
