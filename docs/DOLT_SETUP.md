# Dolt Setup Guide

This guide explains how to set up and use the Dolt database for the Grava project.

## Prerequisites

- [Dolt](https://docs.dolthub.com/introduction/installation) must be installed.

## Initialization

Run the setup script to initialize the Dolt database. This script will configure your user identity (if not already set) and initialize the database in `.grava/dolt/`.

```bash
./scripts/init_dolt.sh
```

## Basic Commands

### Connect to SQL Shell
```bash
cd .grava/dolt
dolt sql
```

### Check Status
```bash
cd .grava/dolt
dolt status
```

### Commit Changes
After making changes to the schema or data via SQL:
```bash
cd .grava/dolt
dolt add .
dolt commit -m "Your commit message"
```

## Rollback and Recovery

### Discard Uncommitted Changes
To discard changes to specific tables:
```bash
cd .grava/dolt
dolt checkout <table_name>
```

To discard all uncommitted changes:
```bash
cd .grava/dolt
dolt reset --hard
```

### Revert to Previous Commit
To revert the database to a specific commit hash:
```bash
cd .grava/dolt
dolt checkout <commit_hash>
```

### Recover Deleted Data (Time Travel)
You can query data as it existed at a specific point in time using standard SQL:
```sql
SELECT * FROM <table_name> AS OF '2023-10-27';
```

## Directory Structure
The database is initialized in `.grava/dolt/`. This directory is ignored by git to prevent conflicts, as Dolt manages its own version control.
