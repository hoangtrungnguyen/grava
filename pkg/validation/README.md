# Package: validation

Path: `github.com/hoangtrungnguyen/grava/pkg/validation`

## Purpose

Centralises input validation for Grava issue fields: type, status, priority,
and date ranges. Provides both allow-list maps and validator functions with
user-friendly error messages.

## Key Types & Functions

- `AllowedIssueTypes` — `map[string]bool` covering `task`, `bug`, `epic`,
  `story`, `feature`, `chore`.
- `AllowedStatuses` — `map[string]bool` covering `open`, `in_progress`,
  `closed`, `blocked`, `tombstone`, `archived`.
- `PriorityMap` — `map[string]int` mapping `critical|high|medium|low|backlog`
  to `0..4`.
- `ValidateIssueType(t string) error` — normalises (`ToLower` + `TrimSpace`)
  and checks membership.
- `ValidateStatus(s string) error` — same pattern for status.
- `ValidatePriority(p string) (int, error)` — returns the integer priority on
  success.
- `ValidateDateRange(fromStr, toStr string) (time.Time, time.Time, error)` —
  parses `YYYY-MM-DD` dates and rejects inverted ranges.

## Dependencies

- Standard library only (`fmt`, `strings`, `time`).

## How It Fits

Every CLI command that creates or filters issues (`grava issue create`,
`grava list`, history queries, sprint reports) calls into this package
before touching the database. Keeping the allow-lists here ensures the SQL
`CHECK` constraints in `pkg/migrate/migrations` and the Go-side validators
agree on the canonical vocabulary.

## Usage

```go
if err := validation.ValidateIssueType(userInput); err != nil {
    return err
}
prio, err := validation.ValidatePriority("high") // returns 1
```
