# Claude Configuration for Grava Project

## Agent Workflows

This project uses custom agent workflows defined in the `.agent/workflows/` directory. Please reference these workflows when working on tasks.

### Available Workflows

- **are-u-ready**: Located at [`.agent/workflows/are-u-ready.md`](.agent/workflows/are-u-ready.md)
  - Protocol for validating readiness before starting tickets
  - Checks context, dependencies, and environment connections
  - Reference the `epic/` and `tracker/` directories for task context

## Project Structure

- `.agent/workflows/`: Custom agent workflows and protocols
- `tracker/`: Historical task tracking and completed work
- `epic/` or `docs/epics/`: Epic definitions and roadmaps
- `docs/`: Project documentation

## Working with Tasks

When working on tickets or tasks:
1. Check the `tracker/` directory for historical context
2. Reference `docs/epics/` for epic-level requirements
3. Follow workflows defined in `.agent/workflows/`
4. Use the Dolt database located at `.grava/dolt/`

## Database

This project uses Dolt as its database substrate. The database directory is `.grava/dolt/`. Use the following connection:
- Command: `dolt --data-dir .grava/dolt sql`
- Connection string: `root@tcp(127.0.0.1:3306)/grava?parseTime=true`
