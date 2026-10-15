package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

type Level string

const (
	LevelDebug Level = "DEBUG"
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
	LevelFatal Level = "FATAL"
)

func (l Level) String() string {
	return string(l)
}

func ToSlogLevel(lvl string) slog.Level {
	switch strings.ToUpper(lvl) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR", "FATAL":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type Logger struct {
	inner *slog.Logger
}

func CreateDefaultLogger(level string) *Logger {
	return NewLogger(os.Stdout, level)
}

func NewLogger(out io.Writer, level string) *Logger {
	opts := &slog.HandlerOptions{
		Level: ToSlogLevel(level),
	}
	handler := slog.NewJSONHandler(out, opts)
	return &Logger{
		inner: slog.New(handler),
	}
}

func (l *Logger) Debug(msg string, args ...any) {
	l.inner.Debug(msg, args...)
}

func (l *Logger) Info(msg string, args ...any) {
	l.inner.Info(msg, args...)
}

func (l *Logger) Warn(msg string, args ...any) {
	l.inner.Warn(msg, args...)
}

func (l *Logger) Error(msg string, args ...any) {
	l.inner.Error(msg, args...)
}

func (l *Logger) Fatal(msg string, args ...any) {
	l.inner.Error(fmt.Sprintf("[FATAL] %s", msg), args...)
	os.Exit(1)
}

func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		inner: l.inner.With(args...),
	}
}
