package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/BadgerOps/airgap/internal/jobsvc"
	"github.com/BadgerOps/airgap/internal/store"
)

type jobJSON struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	Provider  string    `json:"provider"`
	CronExpr  string    `json:"cron_expr"`
	Status    string    `json:"status"`
	LastRun   time.Time `json:"last_run"`
	NextRun   time.Time `json:"next_run"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type jobRequest struct {
	Type     string `json:"type"`
	Provider string `json:"provider"`
	CronExpr string `json:"cron_expr"`
	Status   string `json:"status"`
}

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	providers := []string{}
	if s.store != nil {
		configs, err := s.store.ListProviderConfigs()
		if err != nil {
			s.logger.Warn("failed to list providers for jobs page", "error", err)
		} else {
			for _, cfg := range configs {
				providers = append(providers, cfg.Name)
			}
			sort.Strings(providers)
		}
	}

	defaultCron := ""
	if s.config != nil {
		defaultCron = s.config.Schedule.DefaultCron
	}

	data := map[string]interface{}{
		"Title":       "Jobs",
		"Providers":   providers,
		"DefaultCron": defaultCron,
	}

	s.renderTemplate(w, "templates/jobs.html", data)
}

func (s *Server) handleAPIJobsList(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.store.ListJobs("", 0)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := make([]jobJSON, 0, len(jobs))
	for _, j := range jobs {
		resp = append(resp, toJobJSON(j))
	}

	w.Header().Set("Content-Type", "application/json")
	s.writeJSON(w, resp)
}

func (s *Server) handleAPIJobsCreate(w http.ResponseWriter, r *http.Request) {
	var req jobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	job, err := s.buildJobFromRequest(nil, req)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.store.CreateJob(job); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	s.writeJSON(w, toJobJSON(*job))
}

func (s *Server) handleAPIJobsUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r.PathValue("id"))
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	existing, err := s.store.GetJob(id)
	if err != nil {
		jsonError(w, http.StatusNotFound, "job not found")
		return
	}
	if existing.Status == "running" {
		jsonError(w, http.StatusConflict, "cannot update a running job")
		return
	}

	var req jobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	job, err := s.buildJobFromRequest(existing, req)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	job.ID = existing.ID
	job.CreatedAt = existing.CreatedAt
	job.LastRun = existing.LastRun

	if err := s.store.UpdateJob(job); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	s.writeJSON(w, toJobJSON(*job))
}

func (s *Server) handleAPIJobsDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r.PathValue("id"))
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	job, err := s.store.GetJob(id)
	if err != nil {
		jsonError(w, http.StatusNotFound, "job not found")
		return
	}
	if job.Status == "running" {
		jsonError(w, http.StatusConflict, "cannot delete a running job")
		return
	}

	if err := s.store.DeleteJob(id); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAPIJobsPause(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r.PathValue("id"))
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	job, err := s.store.GetJob(id)
	if err != nil {
		jsonError(w, http.StatusNotFound, "job not found")
		return
	}
	if job.Status == "running" {
		jsonError(w, http.StatusConflict, "cannot pause a running job")
		return
	}

	job.Status = "paused"
	job.UpdatedAt = time.Now()
	if err := s.store.UpdateJob(job); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	s.writeJSON(w, toJobJSON(*job))
}

func (s *Server) handleAPIJobsResume(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r.PathValue("id"))
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	job, err := s.store.GetJob(id)
	if err != nil {
		jsonError(w, http.StatusNotFound, "job not found")
		return
	}
	if job.Status == "running" {
		jsonError(w, http.StatusConflict, "cannot resume a running job")
		return
	}

	nextRun, err := jobsvc.NextRun(job.CronExpr, time.Now())
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid cron expression: "+err.Error())
		return
	}

	job.Status = "scheduled"
	job.NextRun = nextRun
	job.UpdatedAt = time.Now()
	if err := s.store.UpdateJob(job); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	s.writeJSON(w, toJobJSON(*job))
}

func (s *Server) handleAPIJobsRunNow(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r.PathValue("id"))
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	job, err := s.store.GetJob(id)
	if err != nil {
		jsonError(w, http.StatusNotFound, "job not found")
		return
	}
	if job.Status == "running" {
		jsonError(w, http.StatusConflict, "job is already running")
		return
	}

	runCtx, release, err := s.beginOperation(context.Background())
	if err != nil {
		if err == errOperationRunning {
			jsonError(w, http.StatusConflict, "another operation is already running")
			return
		}
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	preservePaused := job.Status == "paused"
	go func(j store.Job) {
		defer release()
		if runErr := s.executeJobAndPersist(runCtx, &j, preservePaused); runErr != nil {
			s.logger.Warn("run-now job failed", "job_id", j.ID, "error", runErr)
		}
	}(*job)

	w.Header().Set("Content-Type", "application/json")
	s.writeJSON(w, map[string]interface{}{
		"status": "started",
		"job":    toJobJSON(*job),
	})
}

func (s *Server) buildJobFromRequest(existing *store.Job, req jobRequest) (*store.Job, error) {
	job := &store.Job{}
	if existing != nil {
		*job = *existing
	}

	if strings.TrimSpace(req.Type) != "" || existing == nil {
		job.Type = jobsvc.NormalizeJobType(req.Type)
	}
	if err := jobsvc.ValidateJobType(job.Type); err != nil {
		return nil, err
	}

	if strings.TrimSpace(req.Provider) != "" || existing == nil {
		job.Provider = jobsvc.NormalizeProviderName(req.Provider)
	}
	if err := jobsvc.ValidateProviderName(s.store, job.Provider); err != nil {
		return nil, err
	}

	if strings.TrimSpace(req.CronExpr) != "" || existing == nil {
		job.CronExpr = strings.TrimSpace(req.CronExpr)
	}
	if err := jobsvc.ValidateCronExpr(job.CronExpr); err != nil {
		return nil, err
	}

	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status == "" && existing == nil {
		status = "scheduled"
	}
	if status != "" {
		switch status {
		case "scheduled", "paused", "completed", "failed":
			job.Status = status
		default:
			return nil, fmt.Errorf("invalid status %q", req.Status)
		}
	}
	if strings.TrimSpace(job.Status) == "" {
		job.Status = "scheduled"
	}

	nextRun, err := jobsvc.NextRun(job.CronExpr, time.Now())
	if err != nil {
		return nil, err
	}
	job.NextRun = nextRun
	now := time.Now()
	if existing == nil {
		job.CreatedAt = now
	}
	job.UpdatedAt = now

	return job, nil
}

func parseJobID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid job id")
	}
	return id, nil
}

func toJobJSON(job store.Job) jobJSON {
	return jobJSON{
		ID:        job.ID,
		Type:      job.Type,
		Provider:  jobsvc.DisplayProviderName(job.Provider),
		CronExpr:  job.CronExpr,
		Status:    job.Status,
		LastRun:   job.LastRun,
		NextRun:   job.NextRun,
		CreatedAt: job.CreatedAt,
		UpdatedAt: job.UpdatedAt,
	}
}
