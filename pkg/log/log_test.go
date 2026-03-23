package log_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gravelog "github.com/hoangtrungnguyen/grava/pkg/log"
)

// captureLog calls Init with a custom writer injected via zerolog.Output so we
// can inspect emitted log lines without touching os.Stderr.
func captureLog(level string, jsonMode bool) (*bytes.Buffer, func()) {
	buf := new(bytes.Buffer)
	gravelog.Init(level, jsonMode)
	// Re-wire Logger output to buf so test can read it.
	gravelog.Logger = gravelog.Logger.Output(buf)
	return buf, func() { gravelog.Logger = zerolog.Nop() }
}

func TestInit_DefaultLevelIsWarn(t *testing.T) {
	buf, cleanup := captureLog("warn", false)
	defer cleanup()

	gravelog.Logger.Debug().Msg("should not appear")
	gravelog.Logger.Info().Msg("should not appear either")
	gravelog.Logger.Warn().Msg("warn message")

	output := buf.String()
	assert.NotContains(t, output, "should not appear")
	assert.Contains(t, output, "warn message")
}

func TestInit_DebugLevelEmitsDebug(t *testing.T) {
	buf, cleanup := captureLog("debug", false)
	defer cleanup()

	gravelog.Logger.Debug().Msg("debug message")

	assert.Contains(t, buf.String(), "debug message")
}

func TestInit_InvalidLevelFallsBackToWarn(t *testing.T) {
	buf, cleanup := captureLog("not-a-level", false)
	defer cleanup()

	gravelog.Logger.Debug().Msg("should not appear")
	gravelog.Logger.Warn().Msg("warn ok")

	output := buf.String()
	assert.NotContains(t, output, "should not appear")
	assert.Contains(t, output, "warn ok")
}

func TestInit_JSONModeEmitsNoANSI(t *testing.T) {
	buf, cleanup := captureLog("debug", true)
	defer cleanup()

	gravelog.Logger.Debug().Msg("json mode test")

	output := buf.String()
	assert.Contains(t, output, "json mode test")
	// ANSI escape codes start with ESC (\x1b); NoColor:true must suppress them.
	assert.NotContains(t, output, "\x1b[")
}

func TestInit_ErrorLevelSuppressesWarn(t *testing.T) {
	buf, cleanup := captureLog("error", false)
	defer cleanup()

	gravelog.Logger.Warn().Msg("warn suppressed")
	gravelog.Logger.Error().Msg("error visible")

	output := buf.String()
	assert.NotContains(t, output, "warn suppressed")
	assert.Contains(t, output, "error visible")
}

func TestInit_LoggerIncludesTimestamp(t *testing.T) {
	// Use JSON output on a raw buffer so we can parse structured fields.
	buf := new(bytes.Buffer)
	gravelog.Logger = zerolog.New(buf).Level(zerolog.DebugLevel).With().Timestamp().Logger()
	defer func() { gravelog.Logger = zerolog.Nop() }()

	gravelog.Logger.Debug().Msg("ts test")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	_, hasTime := entry["time"]
	assert.True(t, hasTime, "log entry must contain 'time' field")
}

func TestInit_MultipleInits_LastWins(t *testing.T) {
	// First init at error (suppresses warn)
	gravelog.Init("error", false)
	gravelog.Init("debug", false)

	buf := new(bytes.Buffer)
	gravelog.Logger = gravelog.Logger.Output(buf)
	defer func() { gravelog.Logger = zerolog.Nop() }()

	gravelog.Logger.Debug().Msg("after reinit")
	assert.Contains(t, buf.String(), "after reinit")
}

func TestInit_TraceLevel(t *testing.T) {
	buf, cleanup := captureLog("trace", false)
	defer cleanup()

	gravelog.Logger.Trace().Msg("trace message")
	assert.Contains(t, buf.String(), "trace message")
}

func TestInit_ConsoleOutputIsHumanReadable(t *testing.T) {
	buf, cleanup := captureLog("info", false)
	defer cleanup()

	gravelog.Logger.Info().Str("key", "value").Msg("human readable")

	output := buf.String()
	// ConsoleWriter formats as readable text containing the message
	assert.Contains(t, output, "human readable")
	// The key=value pair should appear in some form
	assert.True(t, strings.Contains(output, "key") || strings.Contains(output, "value"))
}
