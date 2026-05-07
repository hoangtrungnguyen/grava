package mock

// MockNotifier implements notify.Notifier for testing.
// It captures all Send calls for assertion and returns a configurable Error.
type MockNotifier struct {
	Calls []struct{ Title, Body string }
	// Error is returned by Send when set; nil by default (non-fatal contract).
	Error error
}

// Send captures the call and returns MockNotifier.Error.
func (m *MockNotifier) Send(title, body string) error {
	m.Calls = append(m.Calls, struct{ Title, Body string }{title, body})
	return m.Error
}
