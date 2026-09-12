package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	levelDebug = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Bold(true)
	levelInfo  = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	levelError = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
)

var debugEnabled = new(slog.LevelVar)

// consoleHandler renders slog records as "LEVEL message key=val ..." to
// stderr, colored by level via lipgloss (which no-ops color on non-TTY
// stderr automatically).
type consoleHandler struct{}

func newLogger(debug bool) *slog.Logger {
	if debug {
		debugEnabled.Set(slog.LevelDebug)
	} else {
		debugEnabled.Set(slog.LevelInfo)
	}
	return slog.New(consoleHandler{})
}

func (consoleHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= debugEnabled.Level()
}

func (h consoleHandler) Handle(_ context.Context, record slog.Record) error {
	var style lipgloss.Style
	var label string
	switch {
	case record.Level < slog.LevelInfo:
		style, label = levelDebug, "DEBUG"
	case record.Level < slog.LevelWarn:
		style, label = levelInfo, "INFO"
	default:
		style, label = levelError, "ERROR"
	}
	var fields strings.Builder
	record.Attrs(func(attr slog.Attr) bool {
		fmt.Fprintf(&fields, " %s=%v", attr.Key, attr.Value)
		return true
	})
	fmt.Fprintf(os.Stderr, "%s %s%s\n", style.Render(label), record.Message, fields.String())
	return nil
}

func (h consoleHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h consoleHandler) WithGroup(_ string) slog.Handler      { return h }
