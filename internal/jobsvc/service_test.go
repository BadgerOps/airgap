package jobsvc

import (
	"context"
	"testing"

	"github.com/BadgerOps/airgap/internal/store"
)

func TestNormalizeJobType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"sync", "sync"},
		{"SYNC", "sync"},
		{" Sync ", "sync"},
		{"validate", "validate"},
		{" VALIDATE ", "validate"},
		{"", ""},
	}

	for _, tt := range tests {
		got := NormalizeJobType(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeJobType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestValidateJobType(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"sync", false},
		{"validate", false},
		{"SYNC", false},
		{" validate ", false},
		{"unknown", true},
		{"", true},
		{"import", true},
	}

	for _, tt := range tests {
		err := ValidateJobType(tt.input)
		if tt.wantErr && err == nil {
			t.Errorf("ValidateJobType(%q) expected error", tt.input)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("ValidateJobType(%q) unexpected error: %v", tt.input, err)
		}
	}
}

func TestNormalizeProviderName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"all", ""},
		{"ALL", ""},
		{" All ", ""},
		{"", ""},
		{"epel", "epel"},
		{" epel-main ", "epel-main"},
	}

	for _, tt := range tests {
		got := NormalizeProviderName(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeProviderName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDisplayProviderName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "all"},
		{" ", "all"},
		{"epel", "epel"},
		{"ocp-binaries", "ocp-binaries"},
	}

	for _, tt := range tests {
		got := DisplayProviderName(tt.input)
		if got != tt.want {
			t.Errorf("DisplayProviderName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestValidateProviderNameNilStore(t *testing.T) {
	// "all" (empty) should pass even with nil store
	if err := ValidateProviderName(nil, "all"); err != nil {
		t.Fatalf("expected nil error for 'all' with nil store, got %v", err)
	}

	// Specific provider with nil store should fail
	err := ValidateProviderName(nil, "epel")
	if err == nil {
		t.Fatal("expected error for specific provider with nil store")
	}
}

func TestExecuteJobNilEngine(t *testing.T) {
	job := store.Job{Type: "sync", Provider: ""}
	_, err := ExecuteJob(context.Background(), nil, nil, job, nil)
	if err == nil {
		t.Fatal("expected error for nil engine")
	}
}

func TestExecuteJobInvalidType(t *testing.T) {
	job := store.Job{Type: "bogus", Provider: ""}
	_, err := ExecuteJob(context.Background(), nil, nil, job, nil)
	if err == nil {
		t.Fatal("expected error for invalid job type")
	}
}
