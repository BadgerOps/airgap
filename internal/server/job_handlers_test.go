package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/BadgerOps/airgap/internal/store"
)

func TestHandleAPIJobsCreateRejectsInvalidType(t *testing.T) {
	srv := setupTestServer(t)

	reqBody := `{"type":"export","provider":"all","cron_expr":"*/5 * * * *"}`
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handleAPIJobsCreate(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleAPIJobsCreateAndListNormalizesAllProvider(t *testing.T) {
	srv := setupTestServer(t)

	reqBody := `{"type":"sync","provider":"all","cron_expr":"*/5 * * * *"}`
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleAPIJobsCreate(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created jobJSON
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response failed: %v", err)
	}
	if created.Provider != "all" {
		t.Fatalf("expected provider=all, got %q", created.Provider)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	listW := httptest.NewRecorder()
	srv.handleAPIJobsList(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", listW.Code)
	}
	var jobs []jobJSON
	if err := json.NewDecoder(listW.Body).Decode(&jobs); err != nil {
		t.Fatalf("decode list response failed: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Provider != "all" {
		t.Fatalf("expected list provider=all, got %q", jobs[0].Provider)
	}
}

func TestHandleAPIJobsPauseResumeTransitions(t *testing.T) {
	srv := setupTestServer(t)
	jobID := mustCreateJob(t, srv, "sync", "", "*/5 * * * *", "scheduled", time.Now().Add(time.Minute))

	pauseReq := httptest.NewRequest(http.MethodPost, "/api/jobs/"+strconv.FormatInt(jobID, 10)+"/pause", nil)
	pauseReq.SetPathValue("id", strconv.FormatInt(jobID, 10))
	pauseW := httptest.NewRecorder()
	srv.handleAPIJobsPause(pauseW, pauseReq)
	if pauseW.Code != http.StatusOK {
		t.Fatalf("pause expected 200, got %d: %s", pauseW.Code, pauseW.Body.String())
	}

	job, err := srv.store.GetJob(jobID)
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if job.Status != "paused" {
		t.Fatalf("expected paused status, got %q", job.Status)
	}

	resumeReq := httptest.NewRequest(http.MethodPost, "/api/jobs/"+strconv.FormatInt(jobID, 10)+"/resume", nil)
	resumeReq.SetPathValue("id", strconv.FormatInt(jobID, 10))
	resumeW := httptest.NewRecorder()
	srv.handleAPIJobsResume(resumeW, resumeReq)
	if resumeW.Code != http.StatusOK {
		t.Fatalf("resume expected 200, got %d: %s", resumeW.Code, resumeW.Body.String())
	}

	job, err = srv.store.GetJob(jobID)
	if err != nil {
		t.Fatalf("GetJob failed after resume: %v", err)
	}
	if job.Status != "scheduled" {
		t.Fatalf("expected scheduled status, got %q", job.Status)
	}
}

func TestHandleAPIJobsRunNowConflictWhenBusy(t *testing.T) {
	srv := setupTestServer(t)
	jobID := mustCreateJob(t, srv, "sync", "", "*/5 * * * *", "scheduled", time.Now().Add(time.Minute))

	srv.syncMu.Lock()
	srv.syncRunning = true
	srv.syncMu.Unlock()

	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+strconv.FormatInt(jobID, 10)+"/run", nil)
	req.SetPathValue("id", strconv.FormatInt(jobID, 10))
	w := httptest.NewRecorder()
	srv.handleAPIJobsRunNow(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleAPIJobsDeleteRejectsRunning(t *testing.T) {
	srv := setupTestServer(t)
	jobID := mustCreateJob(t, srv, "sync", "", "*/5 * * * *", "running", time.Now().Add(time.Minute))

	req := httptest.NewRequest(http.MethodDelete, "/api/jobs/"+strconv.FormatInt(jobID, 10), nil)
	req.SetPathValue("id", strconv.FormatInt(jobID, 10))
	w := httptest.NewRecorder()
	srv.handleAPIJobsDelete(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func mustCreateJob(t *testing.T, srv *Server, jobType, provider, cronExpr, status string, nextRun time.Time) int64 {
	t.Helper()
	now := time.Now()
	job := &store.Job{
		Type:      jobType,
		Provider:  provider,
		CronExpr:  cronExpr,
		Status:    status,
		NextRun:   nextRun,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := srv.store.CreateJob(job); err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}
	return job.ID
}
