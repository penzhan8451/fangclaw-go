package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func Setup(level, logFile string) error {
	if err := setLevel(level); err != nil {
		return err
	}

	var writers []io.Writer

	writers = append(writers, zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	})

	if logFile != "" {
		f, err := openLogFile(logFile)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		writers = append(writers, f)
	}

	multi := io.MultiWriter(writers...)
	log.Logger = zerolog.New(multi).With().Timestamp().Logger()

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs

	return nil
}

func SetupFileOnly(level, logFile string) error {
	if err := setLevel(level); err != nil {
		return err
	}

	if logFile == "" {
		log.Logger = zerolog.New(io.Discard).With().Timestamp().Logger()
		return nil
	}

	f, err := openLogFile(logFile)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	log.Logger = zerolog.New(f).With().Timestamp().Logger()

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs

	return nil
}

func setLevel(level string) error {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		return fmt.Errorf("invalid log level '%s': %w", level, err)
	}
	zerolog.SetGlobalLevel(lvl)
	return nil
}

func openLogFile(path string) (*os.File, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	if fi, err := os.Stat(path); err == nil {
		if fi.Size() > 50*1024*1024 {
			if err := os.Truncate(path, 0); err != nil {
				return nil, fmt.Errorf("failed to truncate log file: %w", err)
			}
		}
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return f, nil
}

func DefaultLogFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".fangclaw-go", "daemon.log")
}
