# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /src

# Install build dependencies
RUN apk add --no-cache git

# Download Go module dependencies first (layer-cache friendly)
COPY go.mod go.sum ./
RUN go mod download

# Build the action binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /pb-go-action .

# Install protoc-gen-go plugins into a separate location
RUN GOBIN=/plugins go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.34.2 && \
    GOBIN=/plugins go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.4.0

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:3.20

# protobuf-dev includes protoc and well-known .proto includes
RUN apk add --no-cache protobuf protobuf-dev git ca-certificates

# Copy action binary and protoc plugins
COPY --from=builder /pb-go-action          /usr/local/bin/pb-go-action
COPY --from=builder /plugins/protoc-gen-go      /usr/local/bin/protoc-gen-go
COPY --from=builder /plugins/protoc-gen-go-grpc /usr/local/bin/protoc-gen-go-grpc

ENTRYPOINT ["/usr/local/bin/pb-go-action"]
