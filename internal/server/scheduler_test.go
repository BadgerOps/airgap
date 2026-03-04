package server

import (
	"context"
	"testing"
	"time"

	"github.com/BadgerOps/airgap/internal/store"
)

func TestRunDueJobsExecutesDueJobAndAdvancesSchedule(t *testing.T) {
	srv := setupTestServer(t)
	now := time.Now()
	jobID := mustCreateScheduledJob(t, srv, &store.Job{
		Type:      "validate",
		Provider:  "",
		CronExpr:  "*/5 * * * *",
		Status:    "scheduled",
		NextRun:   now.Add(-time.Minute),
		CreatedAt: now,
		UpdatedAt: now,
	})

	srv.runDueJobs(context.Background())

	job, err := srv.store.GetJob(jobID)
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if job.Status != "completed" {
		t.Fatalf("expected completed status, got %q", job.Status)
	}
	if job.LastRun.IsZero() {
		t.Fatal("expected last_run to be set")
	}
	if !job.NextRun.After(now) {
		t.Fatalf("expected next_run to be advanced, got %s", job.NextRun)
	}
}

func TestRunDueJobsSkipsPausedJobs(t *testing.T) {
	srv := setupTestServer(t)
	now := time.Now()
	jobID := mustCreateScheduledJob(t, srv, &store.Job{
		Type:      "validate",
		Provider:  "",
		CronExpr:  "*/5 * * * *",
		Status:    "paused",
		NextRun:   now.Add(-time.Minute),
		CreatedAt: now,
		UpdatedAt: now,
	})

	srv.runDueJobs(context.Background())

	job, err := srv.store.GetJob(jobID)
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if job.Status != "paused" {
		t.Fatalf("expected paused status, got %q", job.Status)
	}
	if !job.LastRun.IsZero() {
		t.Fatal("expected paused job not to run")
	}
}

func TestRunDueJobsBusyOperationDefersExecution(t *testing.T) {
	srv := setupTestServer(t)
	now := time.Now()
	jobID := mustCreateScheduledJob(t, srv, &store.Job{
		Type:      "validate",
		Provider:  "",
		CronExpr:  "*/5 * * * *",
		Status:    "scheduled",
		NextRun:   now.Add(-time.Minute),
		CreatedAt: now,
		UpdatedAt: now,
	})

	srv.syncMu.Lock()
	srv.syncRunning = true
	srv.syncMu.Unlock()

	srv.runDueJobs(context.Background())

	job, err := srv.store.GetJob(jobID)
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if job.Status != "scheduled" {
		t.Fatalf("expected scheduled status, got %q", job.Status)
	}
	if !job.LastRun.IsZero() {
		t.Fatal("expected job not to run while busy")
	}
}

func TestRunDueJobsFailureSetsFailedAndAdvancesNextRun(t *testing.T) {
	srv := setupTestServer(t)
	now := time.Now()
	jobID := mustCreateScheduledJob(t, srv, &store.Job{
		Type:      "validate",
		Provider:  "missing-provider",
		CronExpr:  "*/5 * * * *",
		Status:    "scheduled",
		NextRun:   now.Add(-time.Minute),
		CreatedAt: now,
		UpdatedAt: now,
	})

	srv.runDueJobs(context.Background())

	job, err := srv.store.GetJob(jobID)
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if job.Status != "failed" {
		t.Fatalf("expected failed status, got %q", job.Status)
	}
	if job.LastRun.IsZero() {
		t.Fatal("expected failed job to update last_run")
	}
	if !job.NextRun.After(now) {
		t.Fatalf("expected next_run to advance even on failure, got %s", job.NextRun)
	}
}

func mustCreateScheduledJob(t *testing.T, srv *Server, job *store.Job) int64 {
	t.Helper()
	if err := srv.store.CreateJob(job); err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}
	return job.ID
}
