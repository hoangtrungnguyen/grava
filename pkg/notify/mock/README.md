# Package: mock

Path: `github.com/hoangtrungnguyen/grava/pkg/notify/mock`

## Purpose

Test double implementation of `notify.Notifier`. Captures every `Send` call
for later assertion and returns a configurable error value.

## Key Types & Functions

- `MockNotifier` — struct with two fields:
  - `Calls []struct{ Title, Body string }` — appended on each `Send`.
  - `Error error` — value returned by `Send`; defaults to `nil` (matching the
    production non-fatal contract).
- `(*MockNotifier).Send(title, body string) error` — records the call and
  returns `Error`.

## Dependencies

- Standard library only. Implicitly satisfies the `notify.Notifier`
  interface defined in `pkg/notify`.

## How It Fits

Used throughout Grava's unit tests wherever a function takes a
`notify.Notifier` dependency (orchestrator, watchdog, merge driver). Tests
inject a `*MockNotifier`, exercise the code, and assert against
`Calls`.

## Usage

```go
m := &mock.MockNotifier{}
runCommand(m)
require.Len(t, m.Calls, 1)
require.Equal(t, "agent dead", m.Calls[0].Title)
```
