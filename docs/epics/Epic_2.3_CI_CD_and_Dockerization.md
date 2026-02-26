# Epic 2.3: CI/CD, Releases, and Dockerization

**Created:** 2026-02-24
**Updated:** 2026-02-25
**Epic:** Epic 2.3: CI/CD, Releases, and Dockerization
**Status:** Planned
**Target Package:** Root, `.github/workflows`

---

## Executive Summary

This epic focuses on professionalizing the development and deployment lifecycle of Grava. By standardizing local development with Docker Compose and implementing automated CI/CD pipelines via GitHub Actions and GoReleaser, we ensure consistent environments and high-quality native binary delivery for our users.

### Key Objectives

1.  **Development Dockerization**: Setup Docker Compose for internal development to easily spin up Dolt alongside test runners without local installation requirements.
2.  **Automated Testing**: Implement GitHub Actions for linting and unit testing (run against ephemeral Dockerized Dolt instances).
3.  **Automated Releases**: Focus CD on GoReleaser to automatically build, cross-compile, and distribute native binaries (macOS, Linux, Windows) on GitHub Releases.

---

## 1. Task Breakdown

### Phase 1: Local Development Environment
*   **[grava-2.3.1] Docker Compose Integration**:
    *   Create `docker-compose.yml` to orchestrate a `grava` testing container alongside a `dolthub/dolt` container running SQL server.
    *   Configure volume persistence for Dolt data and health checks.

### Phase 2: CI Automation
*   **[grava-2.3.2] Lint & Test Workflow**:
    *   Add GitHub Actions workflow for `golangci-lint`.
    *   Add GitHub Actions workflow for running Go tests on every PR using Docker Compose or service containers to spin up a standalone Dolt test service.

### Phase 3: CD & Release Management (GoReleaser)
*   **[grava-2.3.3] GoReleaser Configuration**:
    *   Create `.goreleaser.yml` to define multi-architecture builds (macOS arm64/amd64, Linux, Windows).
    *   Ensure binary is statically linked and optimized.
*   **[grava-2.3.4] Release Automation Workflow**:
    *   Configure GitHub Actions `release.yml` to trigger on tag pushes (`v*`).
    *   Run GoReleaser to automatically attach binaries to GitHub Releases.

---

## 2. Technical Details

### 2.1 GoReleaser Integration
- Create `.goreleaser.yml` optimized for cross-platform Go CLI distribution.
- Support `ldflags` to embed version, commit, and build date into the binary at compile time.

### 2.2 GitHub Actions Stack
- `actions/checkout@v4`
- `actions/setup-go@v5`
- `goreleaser/goreleaser-action@v5`

---

## 3. Verification Plan

*   **Local Docker Test**: Run `docker compose up -d dolt` and verify local `grava` dev cycles work against the isolated `dolt` container.
*   **CI Simulation**: Use `act` or push to a test branch to verify GitHub Actions and linting workflows.
*   **Snapshot Release**: Run `goreleaser build --snapshot --clean` locally to verify the build matrix and binary generation without publishing.

---

## 4. Reference Material
- [Docker and CI/CD Research](artifacts/2026-02-24_Docker_and_CI_CD_Research.md)
