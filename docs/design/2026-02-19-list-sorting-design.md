# Design: Advanced Sorting for `grava list`

Implementing structured sorting functionality for the issue tracker's list command to allow for prioritized and multi-criteria views.

## 1. Objective
Enable users to sort issues returned by the `grava list` command by any table column, with support for multiple sort keys and specific ordering (ascending/descending).

## 2. CLI Interface

### Flag Specification
- **Flag Name**: `--sort`
- **Type**: String
- **Manual format**: `<field>[:<order>][,<field>[:<order>]...]`
- **Default Order**: `asc` (if `:desc` or `:asc` is omitted)

### Supported Field Aliases
To provide a better UX, we map intuitive names to database columns:
- `id` -> `id`
- `title` -> `title`
- `type` -> `issue_type`
- `status` -> `status`
- `priority` -> `priority`
- `created` -> `created_at`
- `updated` -> `updated_at`
- `assignee` -> `assignee`
- `actor` -> `created_by` (optional, based on audit columns)
- `model` -> `agent_model` (optional, based on audit columns)

### Examples
- `grava list --sort priority:asc,created:desc`
- `grava list --sort status` (default order `asc`)
- `grava list --sort updated:desc,priority:asc`

## 3. Implementation Details

### Parsing Logic
1. Split the flag value by comma (`,`).
2. For each fragment, split by colon (`:`) to separate the field from the optional order.
3. Validate the field against a whitelist of valid columns to prevent SQL injection.
4. Validate the order (must be `asc` or `desc`).
5. Construct a list of `ORDER BY` segments.

### SQL Generation
The final `ORDER BY` clause will:
1. Use the explicitly provided sort criteria.
2. **Deterministic Fallback**: Always append `, id ASC` at the end to ensure stable sorting even when other fields are identical.
3. **Default Behavior**: If no `--sort` flag is provided, fall back to the project default: `priority ASC, created_at DESC, id ASC`.

### Error Handling
- Return clear error messages if a user provides an invalid field name.
- Return error if the order is something other than `asc` or `desc`.

## 4. Testing
- **Unit Tests**:
    - Test parser logic with valid/invalid inputs.
    - Test SQL string generation for various combinations.
- **Integration Tests**:
    - Verify that output order in the terminal matches expectations for a small set of mock data.
