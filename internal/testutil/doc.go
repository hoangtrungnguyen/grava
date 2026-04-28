// Package testutil provides shared test fixtures for Grava unit tests.
//
// The package supplies a configurable MockStore implementing dolt.Store, a
// sqlmock-backed *sql.DB factory (NewTestDB) for tests that exercise real
// query strings, and an AssertGravaError helper for checking typed
// gravaerrors.GravaError codes.
//
// MockStore records every ExecContext invocation on its ExecContextCalls
// slice and exposes per-method Fn fields so individual tests can override
// just the behaviour they care about. Methods whose Fn is nil return safe
// zero values; callers that need a *sql.Tx or *sql.Row should either set
// the matching Fn or switch to NewTestDB so sqlmock can drive the
// expectations.
//
// Being under internal/, the package is only importable by code inside the
// grava module — production code should never depend on it.
package testutil
