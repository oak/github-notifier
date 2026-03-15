package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Initialize sets up the global logger with console output and file output (appending to system default log location)
func Initialize() {
	// Determine log file path based on OS
	logFilePath := defaultLogFilePath()
	var file *os.File
	var err error
	if logFilePath != "" {
		// Ensure directory exists
		if err = os.MkdirAll(filepath.Dir(logFilePath), 0o755); err == nil {
			file, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "[logger] Failed to open log file: %v\n", err)
			file = nil
		}
	}

	// Console writer for human-readable output
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}

	// MultiLevelWriter: both file and stdout
	var multi zerolog.LevelWriter
	if file != nil {
		multi = zerolog.MultiLevelWriter(consoleWriter, file)
	} else {
		multi = zerolog.MultiLevelWriter(consoleWriter)
	}

	// Set global logger
	log.Logger = zerolog.New(multi).With().Timestamp().Logger()

	// Set log level from environment or default to Info
	level := os.Getenv("LOG_LEVEL")
	switch level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	log.Info().Str("level", zerolog.GlobalLevel().String()).Str("logFile", logFilePath).Msg("Logger initialized")
}

// defaultLogFilePath returns the default log file path for the current OS
func defaultLogFilePath() string {
	appName := "github-notifier"
	switch runtime.GOOS {
	case "linux":
		stateHome := os.Getenv("XDG_STATE_HOME")
		if stateHome == "" {
			home := os.Getenv("HOME")
			if home == "" {
				return ""
			}
			stateHome = filepath.Join(home, ".local", "state")
		}
		return filepath.Join(stateHome, appName, appName+".log")
	case "darwin":
		home := os.Getenv("HOME")
		if home == "" {
			return ""
		}
		return filepath.Join(home, "Library", "Logs", appName, appName+".log")
	case "windows":
		localAppData := os.Getenv("LocalAppData")
		if localAppData == "" {
			return ""
		}
		return filepath.Join(localAppData, appName, appName+".log")
	default:
		return ""
	}
}
