package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleAPITransfersEmpty(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/transfers", nil)
	w := httptest.NewRecorder()
	srv.handleAPITransfers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var transfers []transferJSON
	if err := json.NewDecoder(w.Body).Decode(&transfers); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(transfers) != 0 {
		t.Fatalf("expected 0 transfers, got %d", len(transfers))
	}
}

func TestHandleAPITransfersNilStore(t *testing.T) {
	srv := setupTestServer(t)
	srv.store = nil

	req := httptest.NewRequest(http.MethodGet, "/api/transfers", nil)
	w := httptest.NewRecorder()
	srv.handleAPITransfers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleAPITransferExportMissingOutputDir(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/transfer/export", strings.NewReader("output_dir="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.handleAPITransferExport(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Output directory is required") {
		t.Fatalf("expected error message, got %q", w.Body.String())
	}
}

func TestHandleAPITransferExportNoProviders(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/transfer/export", strings.NewReader("output_dir=/tmp/test"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.handleAPITransferExport(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "At least one provider") {
		t.Fatalf("expected provider error message, got %q", w.Body.String())
	}
}

func TestHandleAPITransferImportMissingSourceDir(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/transfer/import", strings.NewReader("source_dir="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.handleAPITransferImport(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Source directory is required") {
		t.Fatalf("expected error message, got %q", w.Body.String())
	}
}

func TestWriteTransferFragmentSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	writeTransferFragment(w, true, "Done")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "alert-success") {
		t.Fatalf("expected success class, got %q", body)
	}
	if !strings.Contains(body, "&#10003;") {
		t.Fatalf("expected checkmark, got %q", body)
	}
}

func TestWriteTransferFragmentError(t *testing.T) {
	w := httptest.NewRecorder()
	writeTransferFragment(w, false, "Failed")

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "alert-error") {
		t.Fatalf("expected error class, got %q", body)
	}
	if !strings.Contains(body, "&#10007;") {
		t.Fatalf("expected X mark, got %q", body)
	}
}
