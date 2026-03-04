package main

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/BadgerOps/airgap/internal/jobsvc"
	"github.com/BadgerOps/airgap/internal/store"
	"github.com/spf13/cobra"
)

var (
	jobsListStatus string

	jobsAddType     string
	jobsAddProvider string
	jobsAddCron     string
)

func newJobsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "Manage scheduled jobs",
		Long: `Manage scheduled sync and validation jobs.
Jobs are stored in SQLite and automatically executed when the server scheduler is enabled.`,
	}

	cmd.AddCommand(
		newJobsListCmd(),
		newJobsAddCmd(),
		newJobsPauseCmd(),
		newJobsResumeCmd(),
		newJobsRunNowCmd(),
		newJobsDeleteCmd(),
	)
	return cmd
}

func newJobsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured jobs",
		RunE:  jobsListRun,
	}
	cmd.Flags().StringVar(&jobsListStatus, "status", "", "optional status filter")
	return cmd
}

func jobsListRun(cmd *cobra.Command, args []string) error {
	if globalStore == nil {
		return fmt.Errorf("store not initialized")
	}

	jobs, err := globalStore.ListJobs(strings.TrimSpace(jobsListStatus), 0)
	if err != nil {
		return fmt.Errorf("listing jobs: %w", err)
	}
	if len(jobs) == 0 {
		fmt.Println("No jobs configured.")
		return nil
	}

	fmt.Println("Configured Jobs")
	fmt.Println("===============")
	fmt.Println("")
	fmt.Printf("%-5s %-10s %-20s %-17s %-10s %-19s %-19s\n", "ID", "Type", "Provider", "Cron", "Status", "Last Run", "Next Run")
	fmt.Println(strings.Repeat("-", 110))
	for _, job := range jobs {
		fmt.Printf(
			"%-5d %-10s %-20s %-17s %-10s %-19s %-19s\n",
			job.ID,
			job.Type,
			jobsvc.DisplayProviderName(job.Provider),
			job.CronExpr,
			job.Status,
			formatJobTime(job.LastRun),
			formatJobTime(job.NextRun),
		)
	}
	fmt.Println("")
	return nil
}

func newJobsAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a scheduled job",
		RunE:  jobsAddRun,
	}
	cmd.Flags().StringVar(&jobsAddType, "type", "sync", "job type: sync|validate")
	cmd.Flags().StringVar(&jobsAddProvider, "provider", "all", `provider name or "all"`)
	cmd.Flags().StringVar(&jobsAddCron, "cron", "", "cron expression (5 fields)")
	return cmd
}

func jobsAddRun(cmd *cobra.Command, args []string) error {
	if globalStore == nil {
		return fmt.Errorf("store not initialized")
	}
	if globalCfg == nil {
		return fmt.Errorf("config not loaded")
	}

	if err := jobsvc.ValidateJobType(jobsAddType); err != nil {
		return err
	}

	providerName := jobsvc.NormalizeProviderName(jobsAddProvider)
	if err := jobsvc.ValidateProviderName(globalStore, providerName); err != nil {
		return err
	}

	cronExpr := strings.TrimSpace(jobsAddCron)
	if cronExpr == "" {
		cronExpr = strings.TrimSpace(globalCfg.Schedule.DefaultCron)
	}
	if cronExpr == "" {
		return fmt.Errorf("cron expression is required")
	}
	if err := jobsvc.ValidateCronExpr(cronExpr); err != nil {
		return err
	}

	now := time.Now()
	nextRun, err := jobsvc.NextRun(cronExpr, now)
	if err != nil {
		return err
	}

	job := &store.Job{
		Type:      jobsvc.NormalizeJobType(jobsAddType),
		Provider:  providerName,
		CronExpr:  cronExpr,
		Status:    "scheduled",
		NextRun:   nextRun,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := globalStore.CreateJob(job); err != nil {
		return fmt.Errorf("creating job: %w", err)
	}

	fmt.Printf(
		"Created job %d (%s, provider=%s, cron=%s, next=%s)\n",
		job.ID,
		job.Type,
		jobsvc.DisplayProviderName(job.Provider),
		job.CronExpr,
		formatJobTime(job.NextRun),
	)
	return nil
}

func newJobsPauseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pause JOB_ID",
		Short: "Pause a scheduled job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID, err := parseJobIDArg(args[0])
			if err != nil {
				return err
			}
			job, err := globalStore.GetJob(jobID)
			if err != nil {
				return err
			}
			if job.Status == "running" {
				return fmt.Errorf("cannot pause a running job")
			}
			job.Status = "paused"
			job.UpdatedAt = time.Now()
			if err := globalStore.UpdateJob(job); err != nil {
				return err
			}
			fmt.Printf("Paused job %d\n", job.ID)
			return nil
		},
	}
}

func newJobsResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume JOB_ID",
		Short: "Resume a paused job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID, err := parseJobIDArg(args[0])
			if err != nil {
				return err
			}
			job, err := globalStore.GetJob(jobID)
			if err != nil {
				return err
			}
			if job.Status == "running" {
				return fmt.Errorf("cannot resume a running job")
			}
			nextRun, err := jobsvc.NextRun(job.CronExpr, time.Now())
			if err != nil {
				return err
			}
			job.Status = "scheduled"
			job.NextRun = nextRun
			job.UpdatedAt = time.Now()
			if err := globalStore.UpdateJob(job); err != nil {
				return err
			}
			fmt.Printf("Resumed job %d (next run %s)\n", job.ID, formatJobTime(job.NextRun))
			return nil
		},
	}
}

func newJobsRunNowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run-now JOB_ID",
		Short: "Run a job immediately",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID, err := parseJobIDArg(args[0])
			if err != nil {
				return err
			}
			job, err := globalStore.GetJob(jobID)
			if err != nil {
				return err
			}
			if job.Status == "running" {
				return fmt.Errorf("job is already running")
			}

			if err := executeJobNow(context.Background(), job); err != nil {
				return err
			}

			job, _ = globalStore.GetJob(jobID)
			if job != nil {
				fmt.Printf("Executed job %d: status=%s next=%s\n", job.ID, job.Status, formatJobTime(job.NextRun))
			}
			return nil
		},
	}
}

func newJobsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete JOB_ID",
		Short: "Delete a job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID, err := parseJobIDArg(args[0])
			if err != nil {
				return err
			}
			job, err := globalStore.GetJob(jobID)
			if err != nil {
				return err
			}
			if job.Status == "running" {
				return fmt.Errorf("cannot delete a running job")
			}
			if err := globalStore.DeleteJob(jobID); err != nil {
				return err
			}
			fmt.Printf("Deleted job %d\n", jobID)
			return nil
		},
	}
}

func executeJobNow(ctx context.Context, job *store.Job) error {
	if globalStore == nil {
		return fmt.Errorf("store not initialized")
	}
	if globalEngine == nil {
		return fmt.Errorf("engine not initialized")
	}
	if job == nil {
		return fmt.Errorf("job is required")
	}

	prevStatus := job.Status
	job.Status = "running"
	job.UpdatedAt = time.Now()
	if err := globalStore.UpdateJob(job); err != nil {
		return err
	}

	log := logger
	if log == nil {
		log = slog.Default()
	}
	_, runErr := jobsvc.ExecuteJob(ctx, globalEngine, globalStore, *job, log)
	finishedAt := time.Now()

	if prevStatus == "paused" {
		job.Status = "paused"
	} else if runErr != nil {
		job.Status = "failed"
	} else {
		job.Status = "completed"
	}
	job.LastRun = finishedAt

	nextRun, nextErr := jobsvc.NextRun(job.CronExpr, finishedAt)
	if nextErr != nil {
		job.NextRun = finishedAt.Add(24 * time.Hour)
	} else {
		job.NextRun = nextRun
	}
	job.UpdatedAt = finishedAt

	if err := globalStore.UpdateJob(job); err != nil {
		return err
	}
	return runErr
}

func parseJobIDArg(raw string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid job id %q", raw)
	}
	return id, nil
}

func formatJobTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04:05")
}
