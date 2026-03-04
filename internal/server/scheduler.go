package server

import (
	"context"
	"errors"
	"time"

	"github.com/BadgerOps/airgap/internal/jobsvc"
	"github.com/BadgerOps/airgap/internal/store"
)

const schedulerPollInterval = 15 * time.Second

var errOperationRunning = errors.New("operation already running")

func (s *Server) startScheduler() {
	if s.store == nil || s.engine == nil {
		return
	}
	if s.schedulerCancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.schedulerCancel = cancel
	s.schedulerWG.Add(1)
	go func() {
		defer s.schedulerWG.Done()
		s.schedulerLoop(ctx)
	}()
	s.logger.Info("job scheduler started", "interval", schedulerPollInterval.String())
}

func (s *Server) stopScheduler() {
	if s.schedulerCancel == nil {
		return
	}
	s.schedulerCancel()
	s.schedulerWG.Wait()
	s.schedulerCancel = nil
	s.logger.Info("job scheduler stopped")
}

func (s *Server) schedulerLoop(ctx context.Context) {
	ticker := time.NewTicker(schedulerPollInterval)
	defer ticker.Stop()

	s.runDueJobs(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runDueJobs(ctx)
		}
	}
}

func (s *Server) runDueJobs(ctx context.Context) {
	jobs, err := s.store.ListDueJobs(time.Now(), 20)
	if err != nil {
		s.logger.Warn("failed to list due jobs", "error", err)
		return
	}

	for _, job := range jobs {
		select {
		case <-ctx.Done():
			return
		default:
		}

		runCtx, release, err := s.beginOperation(ctx)
		if err != nil {
			if errors.Is(err, errOperationRunning) {
				// A user-triggered sync/validate is in progress.
				return
			}
			s.logger.Warn("failed to begin scheduled job operation", "job_id", job.ID, "error", err)
			continue
		}

		if runErr := s.executeJobAndPersist(runCtx, &job, false); runErr != nil {
			s.logger.Warn("scheduled job execution failed", "job_id", job.ID, "type", job.Type, "provider", jobsvc.DisplayProviderName(job.Provider), "error", runErr)
		}
		release()
	}
}

// beginOperation acquires the global operation lock.
func (s *Server) beginOperation(parent context.Context) (context.Context, func(), error) {
	s.syncMu.Lock()
	if s.syncRunning {
		s.syncMu.Unlock()
		return nil, nil, errOperationRunning
	}

	runCtx, cancel := context.WithCancel(parent)
	s.syncCancel = cancel
	s.syncRunning = true
	s.syncMu.Unlock()

	release := func() {
		s.syncMu.Lock()
		s.syncRunning = false
		s.syncCancel = nil
		s.syncMu.Unlock()
	}

	return runCtx, release, nil
}

func (s *Server) executeJobAndPersist(ctx context.Context, job *store.Job, preservePaused bool) error {
	if job == nil {
		return errors.New("job is required")
	}

	previousStatus := job.Status
	job.Status = "running"
	job.UpdatedAt = time.Now()
	if err := s.store.UpdateJob(job); err != nil {
		return err
	}

	_, runErr := jobsvc.ExecuteJob(ctx, s.engine, s.store, *job, s.logger)
	finishedAt := time.Now()

	if preservePaused && previousStatus == "paused" {
		job.Status = "paused"
	} else if runErr != nil {
		job.Status = "failed"
	} else {
		job.Status = "completed"
	}

	job.LastRun = finishedAt
	nextRun, nextErr := jobsvc.NextRun(job.CronExpr, finishedAt)
	if nextErr != nil {
		// Cron is validated on write paths; fallback avoids tight-loop retries.
		s.logger.Warn("failed to compute next run for job", "job_id", job.ID, "cron_expr", job.CronExpr, "error", nextErr)
		job.NextRun = finishedAt.Add(24 * time.Hour)
	} else {
		job.NextRun = nextRun
	}
	job.UpdatedAt = finishedAt
	if err := s.store.UpdateJob(job); err != nil {
		return err
	}

	return runErr
}
