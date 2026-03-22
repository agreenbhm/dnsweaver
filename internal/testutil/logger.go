package testutil

import (
	"log/slog"
	"os"
)

// QuietLogger returns a logger that discards all output.
// Use in tests where log output would be noise.
func QuietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError + 1, // Discard everything
	}))
}

// DebugLogger returns a logger that outputs at DEBUG level to stderr.
// Useful for debugging failing tests.
func DebugLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

// ErrorLogger returns a logger that only outputs ERROR and above.
func ErrorLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
}
