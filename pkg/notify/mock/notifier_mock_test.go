package mock

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockNotifier_CapturesCalls(t *testing.T) {
	m := &MockNotifier{}
	err := m.Send("title1", "body1")
	require.NoError(t, err)
	err = m.Send("title2", "body2")
	require.NoError(t, err)

	assert.Len(t, m.Calls, 2)
	assert.Equal(t, "title1", m.Calls[0].Title)
	assert.Equal(t, "body1", m.Calls[0].Body)
	assert.Equal(t, "title2", m.Calls[1].Title)
	assert.Equal(t, "body2", m.Calls[1].Body)
}

func TestMockNotifier_ReturnsConfiguredError(t *testing.T) {
	sentinelErr := errors.New("notifier error")
	m := &MockNotifier{Error: sentinelErr}
	err := m.Send("title", "body")
	assert.Equal(t, sentinelErr, err)
}

func TestMockNotifier_DefaultNoError(t *testing.T) {
	m := &MockNotifier{}
	err := m.Send("title", "body")
	assert.NoError(t, err)
}
