package devlog

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

var (
	logger *log.Logger
	file   *os.File
)

// Init initializes the development logger if enabled.
// If not enabled, the underlying logger remains nil (no-op).
//
// Deprecated: use pkg/log (zerolog) instead.
func Init(enabled bool, logFilePath string) error {
	if !enabled {
		return nil
	}

	if logFilePath == "" {
		// Use default if not provided
		logFilePath = ".grava/dev.log"
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(logFilePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// Open file for appending
	var err error
	file, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	logger = log.New(file, "[DEV] ", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
	return nil
}

// Close closes the underlying log file, if any.
//
// Deprecated: use pkg/log (zerolog) instead.
func Close() error {
	if file != nil {
		err := file.Close()
		file = nil
		logger = nil
		return err
	}
	return nil
}

// Printf logs a formatted message if the logger is enabled.
//
// Deprecated: use pkg/log (zerolog) instead.
func Printf(format string, v ...interface{}) {
	if logger != nil {
		// Call Output with calldepth 2 so it shows the caller of Printf
		_ = logger.Output(2, fmt.Sprintf(format, v...))
	}
}

// Println logs a message with a newline if the logger is enabled.
//
// Deprecated: use pkg/log (zerolog) instead.
func Println(v ...interface{}) {
	if logger != nil {
		// Call Output with calldepth 2 so it shows the caller of Println
		_ = logger.Output(2, fmt.Sprintln(v...))
	}
}
