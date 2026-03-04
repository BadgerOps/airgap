package main

import (
	"context"
	"io"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/BadgerOps/airgap/internal/config"
	"github.com/BadgerOps/airgap/internal/download"
	"github.com/BadgerOps/airgap/internal/engine"
	"github.com/BadgerOps/airgap/internal/provider"
)

func TestJobsLifecycleCommands(t *testing.T) {
	st := newTestStore(t)
	mustCreateProviderConfig(t, st, "provider-a", "epel", true)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.DefaultConfig()
	cfg.Server.DataDir = t.TempDir()
	cfg.Providers = map[string]config.ProviderConfig{
		"provider-a": {"enabled": true},
	}

	reg := provider.NewRegistry()
	reg.RegisterAs("provider-a", &jobsTestProvider{name: "provider-a"})
	eng := engine.NewSyncManager(reg, st, download.NewClient(log), cfg, log)

	origStore := globalStore
	origRegistry := globalRegistry
	origEngine := globalEngine
	origCfg := globalCfg
	origLogger := logger
	t.Cleanup(func() {
		globalStore = origStore
		globalRegistry = origRegistry
		globalEngine = origEngine
		globalCfg = origCfg
		logger = origLogger
	})

	globalStore = st
	globalRegistry = reg
	globalEngine = eng
	globalCfg = cfg
	logger = log

	jobsAddType = "sync"
	jobsAddProvider = "provider-a"
	jobsAddCron = "*/5 * * * *"
	if err := jobsAddRun(nil, nil); err != nil {
		t.Fatalf("jobsAddRun failed: %v", err)
	}

	jobs, err := st.ListJobs("", 0)
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected one job, got %d", len(jobs))
	}
	jobID := jobs[0].ID

	pauseCmd := newJobsPauseCmd()
	if err := pauseCmd.RunE(pauseCmd, []string{strconv.FormatInt(jobID, 10)}); err != nil {
		t.Fatalf("pause failed: %v", err)
	}
	job, err := st.GetJob(jobID)
	if err != nil {
		t.Fatalf("GetJob failed after pause: %v", err)
	}
	if job.Status != "paused" {
		t.Fatalf("expected paused status, got %q", job.Status)
	}

	resumeCmd := newJobsResumeCmd()
	if err := resumeCmd.RunE(resumeCmd, []string{strconv.FormatInt(jobID, 10)}); err != nil {
		t.Fatalf("resume failed: %v", err)
	}
	job, err = st.GetJob(jobID)
	if err != nil {
		t.Fatalf("GetJob failed after resume: %v", err)
	}
	if job.Status != "scheduled" {
		t.Fatalf("expected scheduled status, got %q", job.Status)
	}

	runNowCmd := newJobsRunNowCmd()
	if err := runNowCmd.RunE(runNowCmd, []string{strconv.FormatInt(jobID, 10)}); err != nil {
		t.Fatalf("run-now failed: %v", err)
	}
	job, err = st.GetJob(jobID)
	if err != nil {
		t.Fatalf("GetJob failed after run-now: %v", err)
	}
	if job.Status != "completed" {
		t.Fatalf("expected completed status, got %q", job.Status)
	}
	if job.LastRun.IsZero() {
		t.Fatal("expected last_run to be set")
	}

	deleteCmd := newJobsDeleteCmd()
	if err := deleteCmd.RunE(deleteCmd, []string{strconv.FormatInt(jobID, 10)}); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if _, err := st.GetJob(jobID); err == nil {
		t.Fatal("expected deleted job to be missing")
	}
}

func TestJobsCommandsInvalidID(t *testing.T) {
	pauseCmd := newJobsPauseCmd()
	if err := pauseCmd.RunE(pauseCmd, []string{"abc"}); err == nil {
		t.Fatal("expected invalid id error")
	}
}

func TestJobsAddValidationErrors(t *testing.T) {
	st := newTestStore(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.DefaultConfig()
	cfg.Server.DataDir = t.TempDir()
	cfg.Providers = map[string]config.ProviderConfig{}

	reg := provider.NewRegistry()
	eng := engine.NewSyncManager(reg, st, download.NewClient(log), cfg, log)

	origStore := globalStore
	origRegistry := globalRegistry
	origEngine := globalEngine
	origCfg := globalCfg
	origLogger := logger
	t.Cleanup(func() {
		globalStore = origStore
		globalRegistry = origRegistry
		globalEngine = origEngine
		globalCfg = origCfg
		logger = origLogger
	})

	globalStore = st
	globalRegistry = reg
	globalEngine = eng
	globalCfg = cfg
	logger = log

	jobsAddType = "sync"
	jobsAddProvider = "missing-provider"
	jobsAddCron = "*/5 * * * *"
	if err := jobsAddRun(nil, nil); err == nil {
		t.Fatal("expected unknown provider error")
	}

	mustCreateProviderConfig(t, st, "provider-a", "epel", true)
	jobsAddProvider = "provider-a"
	jobsAddCron = "invalid cron"
	if err := jobsAddRun(nil, nil); err == nil {
		t.Fatal("expected invalid cron error")
	}
}

type jobsTestProvider struct {
	name string
}

func (p *jobsTestProvider) Name() string { return p.name }

func (p *jobsTestProvider) SetName(name string) { p.name = name }

func (p *jobsTestProvider) Type() string { return "test" }

func (p *jobsTestProvider) Configure(cfg provider.ProviderConfig) error { return nil }

func (p *jobsTestProvider) Plan(ctx context.Context) (*provider.SyncPlan, error) {
	return &provider.SyncPlan{
		Provider:   p.name,
		Actions:    nil,
		TotalSize:  0,
		TotalFiles: 0,
		Timestamp:  time.Now(),
	}, nil
}

func (p *jobsTestProvider) Sync(ctx context.Context, plan *provider.SyncPlan, opts provider.SyncOptions) (*provider.SyncReport, error) {
	return &provider.SyncReport{Provider: p.name, StartTime: time.Now(), EndTime: time.Now()}, nil
}

func (p *jobsTestProvider) Validate(ctx context.Context) (*provider.ValidationReport, error) {
	return &provider.ValidationReport{
		Provider:     p.name,
		TotalFiles:   0,
		ValidFiles:   0,
		InvalidFiles: nil,
		Timestamp:    time.Now(),
	}, nil
}
