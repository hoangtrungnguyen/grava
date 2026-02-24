# Research: Dockerization and CI/CD for Grava

**Date:** 2026-02-24
**Topic:** Containerization and Automation Strategy

## 1. Overview
This document outlines the research and strategy for containerizing the Grava project and implementing a CI/CD pipeline.

## 2. Dockerization Strategy

### 2.1 Multi-Stage Build
Grainy Go binaries are best served in minimal containers. A multi-stage Dockerfile will be used:
- **Stage 1 (Build):** Use `golang:1.25-alpine`.
    - Install build dependencies (git, gcc, musl-dev if CGO is needed).
    - Cache Go modules using `go mod download`.
    - Build the static binary.
- **Stage 2 (Runtime):** Use `alpine:latest` or `distroless`.
    - Copy the binary from the build stage.
    - Include only necessary runtime assets.

### 2.2 Database (Dolt) Integration
Grava depends on Dolt. There are two primary approaches for Dockerization:
1.  **Shared Volume:** Run Dolt on the host or in a separate container, and mount the `.grava/dolt` directory as a volume.
2.  **Sidecar/Compose:** Use `docker-compose.yml` to orchestrate a `grava` container and a `dolt` container. 
    - The `dolt` container will run `dolt sql-server`.
    - Grava will connect via TCP (`root@tcp(dolt:3306)/grava`).

### 2.3 Optimization
- Use `.dockerignore` to exclude `vendor/`, `.git/`, and local `.grava/` data.
- Leverage Docker layer caching by copying `go.mod` and `go.sum` before the source code.

## 3. CI/CD Strategy (GitHub Actions)

### 3.1 Continual Integration (CI)
Triggered on every Pull Request and push to `main`:
- **Linting:** Use `golangci-lint-action`.
- **Testing:** Run `go test ./...` with a mock or ephemeral Dolt instance.
- **Build Verification:** Ensure the project compiles across targets.

### 3.2 Continual Deployment/Delivery (CD)
Triggered on tagged releases or manual dispatch:
- **Docker Image Build:** Use `docker/build-push-action`.
- **Registry:** Push to GitHub Container Registry (GHCR) or Docker Hub.
- **Automated Versioning:** Use Git tags to version the images.

## 4. Proposed Steps
1.  Create `Dockerfile` in the root directory.
2.  Create `.dockerignore`.
3.  Create `docker-compose.yml` for local development.
4.  Configure GitHub Actions workflows in `.github/workflows/`.
