# Epic 2.3: CI/CD and Dockerization

**Created:** 2026-02-24
**Epic:** Epic 2.3: CI/CD and Dockerization
**Status:** Planned
**Target Package:** Root, `.github/workflows`

---

## Executive Summary

This epic focuses on professionalizing the development and deployment lifecycle of Grava. By containerizing the application and implementing automated CI/CD pipelines, we ensure consistent environments and high-quality code delivery.

### Key Objectives

1.  **Dockerization**: Create a production-ready, minimal Docker image for Grava.
2.  **Orchestration**: Provide a Docker Compose setup for local development and testing with Dolt.
3.  **Automated Testing**: Implement GitHub Actions for linting and unit testing.
4.  **Automated Delivery**: Implement GitHub Actions for building and pushing Docker images to a registry.

---

## 1. Task Breakdown

### Phase 1: Containerization
*   **[grava-2.3.1] Production Dockerfile**:
    *   Implement a multi-stage Dockerfile using `golang:1.25-alpine` and `alpine`.
    *   Optimize for small image size and fast build times.
*   **[grava-2.3.2] Docker Compose Integration**:
    *   Create `docker-compose.yml` to run Grava alongside a Dolt SQL server.
    *   Configure volume persistence for Dolt data.

### Phase 2: CI Automation
*   **[grava-2.3.3] Lint & Test Workflow**:
    *   Add GitHub Actions workflow for `golangci-lint`.
    *   Add GitHub Actions workflow for running Go tests on every PR.
*   **[grava-2.3.4] Build Verification**:
    *   Ensure the Docker image builds successfully in the CI pipeline.

### Phase 3: CD & Release Management
*   **[grava-2.3.5] Image Registry Integration**:
    *   Configure GitHub Actions to push images to GHCR.
    *   Implement semantic versioning for image tags.

---

## 2. Technical Details

### 2.1 Multi-Stage Dockerfile Layout
- **Build Stage**: 
    - ENV `CGO_ENABLED=0`
    - `go build -o /app/grava ./cmd/grava`
- **Runtime Stage**:
    - `ENTRYPOINT ["/app/grava"]`

### 2.2 GitHub Actions Stack
- `actions/checkout@v4`
- `actions/setup-go@v5`
- `docker/build-push-action@v5`

---

## 3. Verification Plan

*   **Local Docker Test**: Run `docker compose up` and verify `grava` can connect to the `dolt` container.
*   **CI Simulation**: Use `act` or push to a test branch to verify GitHub Actions workflows.
*   **Image Audit**: Inspect the final Docker image size and security vulnerabilities.

---

## 4. Reference Material
- [Docker and CI/CD Research](artifacts/2026-02-24_Docker_and_CI_CD_Research.md)
