# Stage 1: Build
FROM golang:1.26-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

WORKDIR /app

# Copy dependency files first to leverage Docker layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the binary
# We disable CGO for a truly static binary that runs on alpine/scratch
RUN CGO_ENABLED=0 make build

# Stage 2: Final Image
FROM alpine:3.21

# Install runtime dependencies (CA certificates for API calls, Node.js for MCP servers)
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    nodejs \
    npm \
    python3 \
    py3-pip \
    ttyd

# Create a non-root user for security
RUN adduser -D -g '' gllmuser

WORKDIR /home/gllmuser

# Copy the binary from the builder stage
COPY --from=builder /app/dist/gllm /usr/local/bin/gllm

# Set the user
USER gllmuser

# Define the entrypoint
ENTRYPOINT ["gllm"]
