# Stage 1: Build Grava
FROM golang:1.24-bookworm AS builder

# Set the working directory inside the container
WORKDIR /app

# Install dependencies before copying the full source to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application
COPY . .

# Build the Grava binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o grava ./cmd/grava

# Stage 2: Final Image
FROM debian:bookworm-slim

# Prevent prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Install dependencies required for dolt installation and general usage
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    curl \
    ca-certificates \
    git \
    sudo \
    && rm -rf /var/lib/apt/lists/*

# Install Dolt using their official installation script
RUN curl -L https://github.com/dolthub/dolt/releases/latest/download/install.sh | bash

# Copy Grava binary from the builder stage
COPY --from=builder /app/grava /usr/local/bin/grava

# Add the entrypoint script
COPY scripts/docker-entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

# Set up user workspace
WORKDIR /workspace

# Set entrypoint to run initialization before dropping to shell
ENTRYPOINT ["docker-entrypoint.sh"]

# Default command is an interactive shell
CMD ["bash"]
