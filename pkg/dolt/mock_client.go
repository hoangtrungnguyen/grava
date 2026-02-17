package dolt

// MockStore is a mock implementation of Store for testing.
type MockStore struct {
	Sequences map[string]int
}

func NewMockStore() *MockStore {
	return &MockStore{
		Sequences: make(map[string]int),
	}
}

func (m *MockStore) GetNextChildSequence(parentID string) (int, error) {
	m.Sequences[parentID]++
	return m.Sequences[parentID], nil
}

func (m *MockStore) Close() error {
	return nil
}
