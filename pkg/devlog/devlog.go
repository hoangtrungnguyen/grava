// Package devlog is a no-op stub retained for backward compatibility.
//
// All functionality has been replaced by pkg/log (zerolog). Every function in
// this package is a guaranteed no-op — no files are opened, no output is written.
// Do not add new callers. This package will be removed once all references are gone.
package devlog

// Deprecated: use pkg/log (zerolog) instead. No-op stub.
func Init(_ bool, _ string) error { return nil }

// Deprecated: use pkg/log (zerolog) instead. No-op stub.
func Close() error { return nil }

// Deprecated: use pkg/log (zerolog) instead. No-op stub.
func Printf(_ string, _ ...interface{}) {}

// Deprecated: use pkg/log (zerolog) instead. No-op stub.
func Println(_ ...interface{}) {}
