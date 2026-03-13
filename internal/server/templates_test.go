package server

import (
	"testing"
	"time"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}

	for _, tt := range tests {
		got := formatBytes(tt.bytes)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestFormatTime(t *testing.T) {
	t.Run("zero time", func(t *testing.T) {
		if got := formatTime(time.Time{}); got != "-" {
			t.Fatalf("expected '-' for zero time, got %q", got)
		}
	})

	t.Run("valid time", func(t *testing.T) {
		tm := time.Date(2026, 3, 13, 14, 30, 45, 0, time.UTC)
		got := formatTime(tm)
		if got != "2026-03-13 14:30:45" {
			t.Fatalf("expected '2026-03-13 14:30:45', got %q", got)
		}
	})
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{500 * time.Millisecond, "500ms"},
		{0, "0ms"},
		{1500 * time.Millisecond, "1.5s"},
		{45 * time.Second, "45.0s"},
		{90 * time.Second, "1.5m"},
		{30 * time.Minute, "30.0m"},
		{90 * time.Minute, "1.5h"},
		{3 * time.Hour, "3.0h"},
	}

	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestFormatDurationBetween(t *testing.T) {
	t.Run("zero start", func(t *testing.T) {
		got := formatDurationBetween(time.Time{}, time.Now())
		if got != "-" {
			t.Fatalf("expected '-' for zero start, got %q", got)
		}
	})

	t.Run("zero end", func(t *testing.T) {
		got := formatDurationBetween(time.Now(), time.Time{})
		if got != "-" {
			t.Fatalf("expected '-' for zero end, got %q", got)
		}
	})

	t.Run("valid range", func(t *testing.T) {
		start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		end := start.Add(5 * time.Second)
		got := formatDurationBetween(start, end)
		if got != "5.0s" {
			t.Fatalf("expected '5.0s', got %q", got)
		}
	})
}

func TestInitializeTemplateFuncs(t *testing.T) {
	funcs := initializeTemplateFuncs()

	required := []string{"formatBytes", "formatTime", "formatDuration"}
	for _, name := range required {
		if funcs[name] == nil {
			t.Errorf("expected template func %q to be registered", name)
		}
	}
}
