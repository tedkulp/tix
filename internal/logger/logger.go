package logger

import (
	"fmt"
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

// emit applies fields to an event and writes it. Callers pass the variadic
// fields slice straight through; only the first map is used.
func emit(ev *zerolog.Event, msg string, fields []map[string]interface{}) {
	if len(fields) > 0 && fields[0] != nil {
		for k, v := range fields[0] {
			ev = ev.Interface(k, v)
		}
	}

	ev.Msg(msg)
}

// Debug logs a debug message
func Debug(msg string, fields ...map[string]interface{}) {
	if !initialized {
		return
	}

	emit(Logger.Debug(), msg, fields)
}

// Info logs an info message
func Info(msg string, fields ...map[string]interface{}) {
	if !initialized {
		return
	}

	emit(Logger.Info(), msg, fields)
}

// Warn logs a warning message
func Warn(msg string, fields ...map[string]interface{}) {
	if !initialized {
		return
	}

	emit(Logger.Warn(), msg, fields)
}

// Error logs an error message
func Error(msg string, err error, fields ...map[string]interface{}) {
	if !initialized {
		return
	}

	ev := Logger.Error()
	if err != nil {
		ev = ev.Err(err)
	}

	emit(ev, msg, fields)
}
