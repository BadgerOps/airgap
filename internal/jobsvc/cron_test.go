package jobsvc

import (
	"testing"
	"time"
)

func TestValidateCronExpr(t *testing.T) {
	tests := []struct {
		expr    string
		wantErr bool
	}{
		{expr: "*/15 * * * *", wantErr: false},
		{expr: "0 2 * * 0", wantErr: false},
		{expr: "5,35 8-18/2 * * 1-5", wantErr: false},
		{expr: "", wantErr: true},
		{expr: "* * * *", wantErr: true},
		{expr: "61 * * * *", wantErr: true},
		{expr: "* 24 * * *", wantErr: true},
	}

	for _, tt := range tests {
		err := ValidateCronExpr(tt.expr)
		if tt.wantErr && err == nil {
			t.Fatalf("expected error for %q", tt.expr)
		}
		if !tt.wantErr && err != nil {
			t.Fatalf("unexpected error for %q: %v", tt.expr, err)
		}
	}
}

func TestNextRun(t *testing.T) {
	base := time.Date(2026, 3, 4, 10, 7, 45, 0, time.UTC)

	next, err := NextRun("*/15 * * * *", base)
	if err != nil {
		t.Fatalf("NextRun failed: %v", err)
	}
	want := time.Date(2026, 3, 4, 10, 15, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("unexpected next run: got %s want %s", next, want)
	}
}

func TestProviderNormalization(t *testing.T) {
	if got := NormalizeProviderName("all"); got != "" {
		t.Fatalf("expected all => empty, got %q", got)
	}
	if got := NormalizeProviderName(""); got != "" {
		t.Fatalf("expected empty => empty, got %q", got)
	}
	if got := NormalizeProviderName("epel-main"); got != "epel-main" {
		t.Fatalf("unexpected provider normalization: %q", got)
	}
}
