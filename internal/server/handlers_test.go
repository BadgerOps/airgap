package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleRedirectDashboard(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.handleRedirectDashboard(w, req)

	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/dashboard" {
		t.Fatalf("expected redirect to /dashboard, got %q", loc)
	}
}

func TestHandleAPIStatusEmpty(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	srv.handleAPIStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}

	var statuses []ProviderStatusJSON
	if err := json.NewDecoder(w.Body).Decode(&statuses); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

func TestHandleAPIProvidersEmpty(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/providers", nil)
	w := httptest.NewRecorder()
	srv.handleAPIProviders(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleAPISyncMissingProvider(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"provider":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleAPISync(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleAPISyncProviderNotFound(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"provider":"nonexistent"}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleAPISync(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleAPISyncInvalidJSON(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/sync", bytes.NewBufferString(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleAPISync(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestHandleAPISyncHTMXMissingProvider(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/sync", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	srv.handleAPISync(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (HTMX fragment), got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Provider name required") {
		t.Fatalf("expected error message in body, got %q", w.Body.String())
	}
}

func TestHandleAPISyncCancelNoSync(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/sync/cancel", nil)
	w := httptest.NewRecorder()
	srv.handleAPISyncCancel(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "no sync running" {
		t.Fatalf("expected 'no sync running', got %q", resp["status"])
	}
}

func TestHandleAPISyncRunningDefault(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sync/running", nil)
	w := httptest.NewRecorder()
	srv.handleAPISyncRunning(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]bool
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["running"] {
		t.Fatal("expected running=false")
	}
}

func TestHandleAPISyncConflict(t *testing.T) {
	srv := setupTestServer(t)

	// Mark sync as running
	srv.syncMu.Lock()
	srv.syncRunning = true
	srv.syncMu.Unlock()

	body := `{"provider":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleAPISync(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}

	// Reset
	srv.syncMu.Lock()
	srv.syncRunning = false
	srv.syncMu.Unlock()
}

func TestHandleAPISyncFailuresMissingProvider(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sync/failures", nil)
	w := httptest.NewRecorder()
	srv.handleAPISyncFailures(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleAPISyncFailuresEmpty(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sync/failures?provider=epel", nil)
	w := httptest.NewRecorder()
	srv.handleAPISyncFailures(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleAPISyncRetryMissingProvider(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"provider":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync/retry", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleAPISyncRetry(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleAPISyncRetryConflict(t *testing.T) {
	srv := setupTestServer(t)

	srv.syncMu.Lock()
	srv.syncRunning = true
	srv.syncMu.Unlock()

	body := `{"provider":"epel"}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync/retry", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleAPISyncRetry(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}

	srv.syncMu.Lock()
	srv.syncRunning = false
	srv.syncMu.Unlock()
}

func TestHandleAPISyncRetryNoFailures(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"provider":"epel"}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync/retry", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleAPISyncRetry(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "no_failures" {
		t.Fatalf("expected 'no_failures', got %q", resp["status"])
	}
}

func TestHandleAPIValidateMissingProvider(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"provider":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/validate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleAPIValidate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleAPIValidateConflict(t *testing.T) {
	srv := setupTestServer(t)

	srv.syncMu.Lock()
	srv.syncRunning = true
	srv.syncMu.Unlock()

	body := `{"provider":"epel"}`
	req := httptest.NewRequest(http.MethodPost, "/api/validate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleAPIValidate(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}

	srv.syncMu.Lock()
	srv.syncRunning = false
	srv.syncMu.Unlock()
}

func TestHandleAPIScanMissingProvider(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"provider":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/scan", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleAPIScan(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleAPIScanConflict(t *testing.T) {
	srv := setupTestServer(t)

	srv.syncMu.Lock()
	srv.syncRunning = true
	srv.syncMu.Unlock()

	body := `{"provider":"epel"}`
	req := httptest.NewRequest(http.MethodPost, "/api/scan", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleAPIScan(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}

	srv.syncMu.Lock()
	srv.syncRunning = false
	srv.syncMu.Unlock()
}

func TestHandleProviderDetailNotFound(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/providers/nonexistent", nil)
	req.SetPathValue("name", "nonexistent")
	w := httptest.NewRecorder()
	srv.handleProviderDetail(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleProviderDetailMissingName(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/providers/", nil)
	// No path value set
	w := httptest.NewRecorder()
	srv.handleProviderDetail(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestParseSyncRequestJSON(t *testing.T) {
	body := `{"provider":"epel","dry_run":true,"force":false,"max_workers":8}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	parsed, err := parseSyncRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Provider != "epel" {
		t.Fatalf("expected provider 'epel', got %q", parsed.Provider)
	}
	if !parsed.DryRun {
		t.Fatal("expected dry_run=true")
	}
	if parsed.MaxWorkers != 8 {
		t.Fatalf("expected max_workers=8, got %d", parsed.MaxWorkers)
	}
}

func TestParseSyncRequestForm(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader("provider=ocp&dry_run=true"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	parsed, err := parseSyncRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Provider != "ocp" {
		t.Fatalf("expected provider 'ocp', got %q", parsed.Provider)
	}
	if !parsed.DryRun {
		t.Fatal("expected dry_run=true")
	}
}

func TestIsHTMX(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if isHTMX(req) {
		t.Fatal("expected false for request without HX-Request header")
	}

	req.Header.Set("HX-Request", "true")
	if !isHTMX(req) {
		t.Fatal("expected true for request with HX-Request header")
	}
}

func TestWriteSyncFragmentSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	writeSyncFragment(w, true, "All good")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "alert-success") {
		t.Fatalf("expected alert-success class, got %q", body)
	}
	if !strings.Contains(body, "All good") {
		t.Fatalf("expected message in body, got %q", body)
	}
}

func TestWriteSyncFragmentError(t *testing.T) {
	w := httptest.NewRecorder()
	writeSyncFragment(w, false, "Something failed")

	body := w.Body.String()
	if !strings.Contains(body, "alert-error") {
		t.Fatalf("expected alert-error class, got %q", body)
	}
}

func TestHandleAPISyncFailuresResolveMissingProvider(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"provider":"","all":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync/failures/resolve", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleAPISyncFailuresResolve(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleAPISyncRetryInvalidJSON(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/sync/retry", bytes.NewBufferString(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleAPISyncRetry(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleAPIValidateInvalidJSON(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/validate", bytes.NewBufferString(`not-json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleAPIValidate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleAPIScanFormEncoded(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/scan", strings.NewReader("provider="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.handleAPIScan(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty provider form value, got %d", w.Code)
	}
}
