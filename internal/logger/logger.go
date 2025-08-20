package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	// Logger is the global logger instance
	Logger zerolog.Logger

	// Initialized flag to check if logger has been initialized
	initialized bool = false
)

// InitLogger initializes the global logger with the given verbosity level
func InitLogger(verboseCount int) {
	// Set up console writer with color
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.DateTime, NoColor: false}

	// Set caller marshal function to show short filename and line number
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		return filepath.Base(file) + ":" + strconv.Itoa(line)
	}

	// Set level formatter to show level in uppercase
	output.FormatLevel = func(i interface{}) string {
		return strings.ToUpper(fmt.Sprintf("| %-6s|", i))
	}

	// Set global logger
	Logger = zerolog.New(output).With().Timestamp().Caller().Logger()

	// Set log level based on verbose count
	// 0: WARN (default)
	// 1: INFO (-v)
	// 2+: DEBUG (-vv or more)
	switch verboseCount {
	case 0:
		Logger = Logger.Level(zerolog.WarnLevel)
	case 1:
		Logger = Logger.Level(zerolog.InfoLevel)
	default: // 2 or more
		Logger = Logger.Level(zerolog.DebugLevel)
	}

	// Replace the global logger
	log.Logger = Logger

	// Mark as initialized
	initialized = true
}

// IsInitialized returns true if the logger has been initialized
func IsInitialized() bool {
	return initialized
}

// Debug logs a debug message
func Debug(msg string, fields ...map[string]interface{}) {
	// Do nothing if logger is not initialized
	if !initialized {
		return
	}

	event := Logger.Debug().Str("level", "debug")

	if len(fields) > 0 && fields[0] != nil {
		for k, v := range fields[0] {
			event = event.Interface(k, v)
		}
	}

	event.Msg(msg)
}

// Info logs an info message
func Info(msg string, fields ...map[string]interface{}) {
	// Do nothing if logger is not initialized
	if !initialized {
		return
	}

	event := Logger.Info().Str("level", "info")

	if len(fields) > 0 && fields[0] != nil {
		for k, v := range fields[0] {
			event = event.Interface(k, v)
		}
	}

	event.Msg(msg)
}

// Warn logs a warning message
func Warn(msg string, fields ...map[string]interface{}) {
	// Do nothing if logger is not initialized
	if !initialized {
		return
	}

	event := Logger.Warn().Str("level", "warn")

	if len(fields) > 0 && fields[0] != nil {
		for k, v := range fields[0] {
			event = event.Interface(k, v)
		}
	}

	event.Msg(msg)
}

// Error logs an error message
func Error(msg string, err error, fields ...map[string]interface{}) {
	// Do nothing if logger is not initialized
	if !initialized {
		return
	}

	event := Logger.Error().Str("level", "error")

	if err != nil {
		event = event.Err(err)
	}

	if len(fields) > 0 && fields[0] != nil {
		for k, v := range fields[0] {
			event = event.Interface(k, v)
		}
	}

	event.Msg(msg)
}

// Fatal logs a fatal message and exits the application
func Fatal(msg string, err error, fields ...map[string]interface{}) {
	// If logger is not initialized, just print to stderr and exit
	if !initialized {
		if err != nil {
			os.Stderr.WriteString(msg + ": " + err.Error() + "\n")
		} else {
			os.Stderr.WriteString(msg + "\n")
		}
		os.Exit(1)
	}

	event := Logger.Fatal().Str("level", "fatal")

	if err != nil {
		event = event.Err(err)
	}

	if len(fields) > 0 && fields[0] != nil {
		for k, v := range fields[0] {
			event = event.Interface(k, v)
		}
	}

	event.Msg(msg)
}

// Writer returns a writer that can be used to pipe logs
func Writer(level zerolog.Level) io.Writer {
	// Do nothing if logger is not initialized
	if !initialized {
		return os.Stdout
	}

	return Logger.Level(level)
}
