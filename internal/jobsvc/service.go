package jobsvc

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/BadgerOps/airgap/internal/engine"
	"github.com/BadgerOps/airgap/internal/provider"
	"github.com/BadgerOps/airgap/internal/store"
)

const (
	JobTypeSync     = "sync"
	JobTypeValidate = "validate"
)

// JobExecutionResult summarizes an executed job.
type JobExecutionResult struct {
	InvalidFiles int
	Message      string
}

// NormalizeJobType normalizes job type input.
func NormalizeJobType(t string) string {
	return strings.ToLower(strings.TrimSpace(t))
}

// ValidateJobType validates allowed job types.
func ValidateJobType(t string) error {
	switch NormalizeJobType(t) {
	case JobTypeSync, JobTypeValidate:
		return nil
	default:
		return fmt.Errorf("invalid job type %q: must be one of %q, %q", t, JobTypeSync, JobTypeValidate)
	}
}

// NormalizeProviderName normalizes provider input; "all" is represented as empty string in DB.
func NormalizeProviderName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || strings.EqualFold(name, "all") {
		return ""
	}
	return name
}

// DisplayProviderName converts internal provider representation to display text.
func DisplayProviderName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "all"
	}
	return name
}

// ValidateProviderName ensures provider is either "all" (empty) or an existing configured provider.
func ValidateProviderName(st *store.Store, providerName string) error {
	providerName = NormalizeProviderName(providerName)
	if providerName == "" {
		return nil
	}
	if st == nil {
		return fmt.Errorf("store is required to validate provider")
	}
	if _, err := st.GetProviderConfig(providerName); err != nil {
		return fmt.Errorf("unknown provider %q", providerName)
	}
	return nil
}

// ExecuteJob executes one sync/validate job.
func ExecuteJob(
	ctx context.Context,
	eng *engine.SyncManager,
	st *store.Store,
	job store.Job,
	logger *slog.Logger,
) (JobExecutionResult, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if eng == nil {
		return JobExecutionResult{}, fmt.Errorf("sync engine is required")
	}

	jobType := NormalizeJobType(job.Type)
	if err := ValidateJobType(jobType); err != nil {
		return JobExecutionResult{}, err
	}

	providerName := NormalizeProviderName(job.Provider)
	result := JobExecutionResult{}

	switch jobType {
	case JobTypeSync:
		opts := provider.SyncOptions{MaxWorkers: 4}
		if providerName == "" {
			reports, err := eng.SyncAll(ctx, opts)
			result.Message = fmt.Sprintf("sync completed for %d provider(s)", len(reports))
			if err != nil {
				return result, fmt.Errorf("sync all failed: %w", err)
			}
			return result, nil
		}

		report, err := eng.SyncProvider(ctx, providerName, opts)
		if err != nil {
			return result, fmt.Errorf("sync failed for provider %q: %w", providerName, err)
		}
		result.Message = fmt.Sprintf(
			"sync completed for %s (downloaded=%d skipped=%d deleted=%d failed=%d)",
			providerName, report.Downloaded, report.Skipped, report.Deleted, len(report.Failed),
		)
		return result, nil

	case JobTypeValidate:
		if providerName == "" {
			reports, err := eng.ValidateAll(ctx)
			invalidCount := persistValidationReports(st, reports, logger)
			result.InvalidFiles = invalidCount
			if err != nil {
				return result, fmt.Errorf("validate all failed: %w", err)
			}
			if invalidCount > 0 {
				return result, fmt.Errorf("validation failed: %d invalid file(s)", invalidCount)
			}
			result.Message = fmt.Sprintf("validation passed for %d provider(s)", len(reports))
			return result, nil
		}

		report, err := eng.ValidateProvider(ctx, providerName)
		if err != nil {
			return result, fmt.Errorf("validation failed for provider %q: %w", providerName, err)
		}
		invalidCount := persistValidationReport(st, providerName, report, logger)
		result.InvalidFiles = invalidCount
		if invalidCount > 0 {
			return result, fmt.Errorf("validation failed for %q: %d invalid file(s)", providerName, invalidCount)
		}
		result.Message = fmt.Sprintf("validation passed for %s (%d files)", providerName, report.TotalFiles)
		return result, nil
	}

	return result, fmt.Errorf("unsupported job type %q", jobType)
}

func persistValidationReports(st *store.Store, reports map[string]*provider.ValidationReport, logger *slog.Logger) int {
	if st == nil {
		return 0
	}
	total := 0
	for providerName, report := range reports {
		if report == nil {
			continue
		}
		total += persistValidationReport(st, providerName, report, logger)
	}
	return total
}

func persistValidationReport(st *store.Store, providerName string, report *provider.ValidationReport, logger *slog.Logger) int {
	if st == nil || report == nil || len(report.InvalidFiles) == 0 {
		return 0
	}

	now := time.Now()
	count := 0
	for _, inv := range report.InvalidFiles {
		errText := "validation: checksum mismatch"
		if strings.EqualFold(inv.Actual, "missing") {
			errText = "validation: file missing"
		}
		rec := &store.FailedFileRecord{
			Provider:         providerName,
			FilePath:         inv.Path,
			URL:              inv.URL,
			DestPath:         inv.LocalPath,
			ExpectedChecksum: inv.Expected,
			ExpectedSize:     inv.Size,
			Error:            errText,
			RetryCount:       0,
			FirstFailure:     now,
			LastFailure:      now,
			Resolved:         false,
		}

		if err := st.AddFailedFile(rec); err != nil {
			logger.Warn("failed to persist validation failure", "provider", providerName, "path", inv.Path, "error", err)
			continue
		}
		count++
	}
	return count
}
