# Dolt Setup Guide

This guide explains how to set up and use the Dolt database for the Grava project.

## Prerequisites

- [Dolt](https://docs.dolthub.com/introduction/installation) must be installed.

## Initialization

Run the setup script to initialize the Dolt database. This script will configure your user identity (if not already set) and initialize the database in `.grava/dolt/`.

```bash
./scripts/init_dolt.sh
```

## Connecting to the Database

### Method 1: CLI (Interactive Shell)
The simplest way to query the database is via the `dolt sql` command.
```bash
cd .grava/dolt
dolt sql
# or execute a single query:
dolt sql -q "SELECT * FROM issues;"
```

### Method 2: SQL Server (For Clients/Apps)
To connect via a GUI client (DBeaver, DataGrip) or application code, start the Dolt SQL server. We've provided a helper script:

```bash
./scripts/start_dolt_server.sh
```

This starts a MySQL-compatible server on port `3306`.
**Connection Details:**
-   **Host**: `127.0.0.1`
-   **Port**: `3306`
-   **User**: `root`
-   **Password**: (empty)
-   **Database**: `grava` (or `dolt_repo` if browsing system tables)

### Stopping the Server
To stop the server:
```bash
./scripts/stop_dolt_server.sh
```
Or, if you are running it in a terminal foreground, simply press `Ctrl+C`.

### Method 3: MySQL Client
If you have the `mysql` CLI installed:
```bash
mysql -h 127.0.0.1 -P 3306 -u root
```

## Directory Structure
The database is initialized in `.grava/dolt/`. This directory is ignored by git to prevent conflicts, as Dolt manages its own version control.
