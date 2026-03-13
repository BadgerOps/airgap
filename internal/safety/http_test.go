package safety

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestNewHTTPClient(t *testing.T) {
	t.Run("default timeout", func(t *testing.T) {
		c := NewHTTPClient(0)
		if c.Timeout != 60*time.Second {
			t.Fatalf("expected 60s default timeout, got %v", c.Timeout)
		}
	})

	t.Run("negative timeout uses default", func(t *testing.T) {
		c := NewHTTPClient(-1)
		if c.Timeout != 60*time.Second {
			t.Fatalf("expected 60s default timeout for negative, got %v", c.Timeout)
		}
	})

	t.Run("custom timeout", func(t *testing.T) {
		c := NewHTTPClient(30 * time.Second)
		if c.Timeout != 30*time.Second {
			t.Fatalf("expected 30s timeout, got %v", c.Timeout)
		}
	})
}

func TestValidateHTTPURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{name: "valid http", raw: "http://example.com/path"},
		{name: "valid https", raw: "https://example.com/path"},
		{name: "https with port", raw: "https://example.com:8080/path"},
		{name: "ftp rejected", raw: "ftp://example.com/file", wantErr: "unsupported URL scheme"},
		{name: "empty scheme", raw: "://example.com", wantErr: "invalid URL"},
		{name: "no host", raw: "http://", wantErr: "URL host is required"},
		{name: "userinfo rejected", raw: "https://user:pass@example.com", wantErr: "userinfo is not allowed"},
		{name: "file scheme rejected", raw: "file:///etc/passwd", wantErr: "unsupported URL scheme"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := ValidateHTTPURL(tt.raw)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if u == nil {
				t.Fatal("expected non-nil URL")
			}
		})
	}
}

func TestIsLoopbackHost(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		loopback bool
	}{
		{name: "localhost", host: "localhost", loopback: true},
		{name: "subdomain.localhost", host: "sub.localhost", loopback: true},
		{name: "127.0.0.1", host: "127.0.0.1", loopback: true},
		{name: "127.0.0.2", host: "127.0.0.2", loopback: true},
		{name: "::1", host: "[::1]", loopback: true},
		{name: "external IP", host: "8.8.8.8", loopback: false},
		{name: "external hostname", host: "example.com", loopback: false},
		{name: "10.0.0.1 private", host: "10.0.0.1", loopback: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := url.Parse("http://" + tt.host + "/path")
			got := IsLoopbackHost(u)
			if got != tt.loopback {
				t.Fatalf("IsLoopbackHost(%q) = %v, want %v", tt.host, got, tt.loopback)
			}
		})
	}
}

func TestReadAllWithLimitInvalid(t *testing.T) {
	_, err := ReadAllWithLimit(strings.NewReader("data"), 0)
	if err == nil {
		t.Fatal("expected error for zero limit")
	}

	_, err = ReadAllWithLimit(strings.NewReader("data"), -5)
	if err == nil {
		t.Fatal("expected error for negative limit")
	}
}

func TestReadAllWithLimitExact(t *testing.T) {
	data, err := ReadAllWithLimit(strings.NewReader("12345"), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "12345" {
		t.Fatalf("expected '12345', got %q", string(data))
	}
}

func TestReadAllWithLimitEmpty(t *testing.T) {
	data, err := ReadAllWithLimit(strings.NewReader(""), 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("expected empty data, got %d bytes", len(data))
	}
}
