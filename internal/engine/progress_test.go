package engine

import (
	"sync"
	"testing"
	"time"
)

func TestNewSyncTracker(t *testing.T) {
	tracker := NewSyncTracker("epel")
	snap := tracker.Snapshot()

	if snap.Provider != "epel" {
		t.Fatalf("expected provider 'epel', got %q", snap.Provider)
	}
	if snap.Phase != PhasePlanning {
		t.Fatalf("expected phase %q, got %q", PhasePlanning, snap.Phase)
	}
	if snap.TotalFiles != 0 || snap.CompletedFiles != 0 || snap.FailedFiles != 0 {
		t.Fatal("expected all counters to be zero on new tracker")
	}
}

func TestSetPhase(t *testing.T) {
	tracker := NewSyncTracker("test")

	phases := []SyncPhase{PhaseDownloading, PhaseComplete, PhaseFailed, PhaseCancelled}
	for _, p := range phases {
		tracker.SetPhase(p)
		snap := tracker.Snapshot()
		if snap.Phase != p {
			t.Fatalf("expected phase %q, got %q", p, snap.Phase)
		}
	}
}

func TestSetTotals(t *testing.T) {
	tracker := NewSyncTracker("test")
	tracker.SetTotals(100, 1024*1024)

	snap := tracker.Snapshot()
	if snap.TotalFiles != 100 {
		t.Fatalf("expected TotalFiles 100, got %d", snap.TotalFiles)
	}
	if snap.TotalBytes != 1024*1024 {
		t.Fatalf("expected TotalBytes %d, got %d", 1024*1024, snap.TotalBytes)
	}
}

func TestSetMessage(t *testing.T) {
	tracker := NewSyncTracker("test")
	tracker.SetMessage("syncing...")

	snap := tracker.Snapshot()
	if snap.Message != "syncing..." {
		t.Fatalf("expected message %q, got %q", "syncing...", snap.Message)
	}
}

func TestFileCompleted(t *testing.T) {
	tracker := NewSyncTracker("test")
	tracker.SetTotals(3, 3000)

	tracker.FileCompleted("/a.rpm", 1000)
	tracker.FileCompleted("/b.rpm", 2000)

	snap := tracker.Snapshot()
	if snap.CompletedFiles != 2 {
		t.Fatalf("expected 2 completed files, got %d", snap.CompletedFiles)
	}
	if snap.BytesDownloaded != 3000 {
		t.Fatalf("expected 3000 bytes downloaded, got %d", snap.BytesDownloaded)
	}
	if len(snap.RecentEvents) != 2 {
		t.Fatalf("expected 2 recent events, got %d", len(snap.RecentEvents))
	}
	// Most recent event should be first
	if snap.RecentEvents[0].Path != "/b.rpm" {
		t.Fatalf("expected most recent event first, got %q", snap.RecentEvents[0].Path)
	}
	if snap.RecentEvents[0].Status != "completed" {
		t.Fatalf("expected status 'completed', got %q", snap.RecentEvents[0].Status)
	}
}

func TestFileFailed(t *testing.T) {
	tracker := NewSyncTracker("test")
	tracker.SetTotals(2, 2000)

	tracker.FileFailed("/c.rpm", "404 not found")

	snap := tracker.Snapshot()
	if snap.FailedFiles != 1 {
		t.Fatalf("expected 1 failed file, got %d", snap.FailedFiles)
	}
	if len(snap.RecentEvents) != 1 {
		t.Fatalf("expected 1 recent event, got %d", len(snap.RecentEvents))
	}
	if snap.RecentEvents[0].Status != "failed" {
		t.Fatalf("expected status 'failed', got %q", snap.RecentEvents[0].Status)
	}
	if snap.RecentEvents[0].Error != "404 not found" {
		t.Fatalf("expected error message, got %q", snap.RecentEvents[0].Error)
	}
}

func TestFileSkipped(t *testing.T) {
	tracker := NewSyncTracker("test")
	tracker.SetTotals(5, 5000)

	tracker.FileSkipped()
	tracker.FileSkipped()

	snap := tracker.Snapshot()
	if snap.SkippedFiles != 2 {
		t.Fatalf("expected 2 skipped files, got %d", snap.SkippedFiles)
	}
}

func TestSetSkippedFiles(t *testing.T) {
	tracker := NewSyncTracker("test")
	tracker.SetTotals(10, 10000)

	tracker.SetSkippedFiles(7)

	snap := tracker.Snapshot()
	if snap.SkippedFiles != 7 {
		t.Fatalf("expected 7 skipped files, got %d", snap.SkippedFiles)
	}
}

func TestAddRetries(t *testing.T) {
	tracker := NewSyncTracker("test")

	tracker.AddRetries(3)
	tracker.AddRetries(2)

	snap := tracker.Snapshot()
	if snap.TotalRetries != 5 {
		t.Fatalf("expected 5 total retries, got %d", snap.TotalRetries)
	}
}

func TestPercentProgress(t *testing.T) {
	tracker := NewSyncTracker("test")
	tracker.SetTotals(4, 4000)

	// 2 of 4 completed = 50%
	tracker.FileCompleted("/a", 1000)
	tracker.FileCompleted("/b", 1000)

	snap := tracker.Snapshot()
	if snap.Percent < 49.0 || snap.Percent > 51.0 {
		t.Fatalf("expected ~50%%, got %.1f%%", snap.Percent)
	}
}

func TestPercentAllSkipped(t *testing.T) {
	tracker := NewSyncTracker("test")
	tracker.SetTotals(3, 3000)
	tracker.SetSkippedFiles(3)

	snap := tracker.Snapshot()
	if snap.Percent != 100 {
		t.Fatalf("expected 100%% when all files skipped, got %.1f%%", snap.Percent)
	}
}

func TestPercentWithActiveFiles(t *testing.T) {
	tracker := NewSyncTracker("test")
	tracker.SetTotals(2, 2000)

	// Simulate an active file at 50% download with forced throttle bypass
	tracker.mu.Lock()
	tracker.files["/active.rpm"] = &FileProgress{
		Path:            "/active.rpm",
		BytesDownloaded: 500,
		TotalBytes:      1000,
	}
	tracker.bytesDownloaded = 500
	tracker.mu.Unlock()

	snap := tracker.Snapshot()
	// active file contributes 0.5 out of 2 work items = 25%
	if snap.Percent < 24.0 || snap.Percent > 26.0 {
		t.Fatalf("expected ~25%% with one active file at 50%%, got %.1f%%", snap.Percent)
	}
}

func TestRecentEventsCapAt20(t *testing.T) {
	tracker := NewSyncTracker("test")
	tracker.SetTotals(25, 25000)

	for i := 0; i < 25; i++ {
		tracker.FileCompleted("/file"+string(rune('a'+i)), 1000)
	}

	snap := tracker.Snapshot()
	if len(snap.RecentEvents) != 20 {
		t.Fatalf("expected recent events capped at 20, got %d", len(snap.RecentEvents))
	}
}

func TestWaitSignal(t *testing.T) {
	tracker := NewSyncTracker("test")

	waitCh := tracker.Wait()

	// Signal should not be ready yet
	select {
	case <-waitCh:
		t.Fatal("wait channel should not be closed before an update")
	default:
	}

	// Trigger an update
	tracker.SetMessage("update")

	// Now the channel should be closed
	select {
	case <-waitCh:
		// expected
	case <-time.After(time.Second):
		t.Fatal("wait channel should be closed after update")
	}

	// Getting a new wait channel should give a fresh one
	newCh := tracker.Wait()
	select {
	case <-newCh:
		t.Fatal("new wait channel should not be closed")
	default:
	}
}

func TestConcurrentUpdates(t *testing.T) {
	tracker := NewSyncTracker("concurrent")
	tracker.SetTotals(100, 100000)
	tracker.SetPhase(PhaseDownloading)

	var wg sync.WaitGroup

	// 10 goroutines completing files
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				path := "/file" + string(rune('A'+n)) + string(rune('0'+j))
				tracker.FileCompleted(path, 1000)
			}
		}(i)
	}

	wg.Wait()

	snap := tracker.Snapshot()
	if snap.CompletedFiles != 100 {
		t.Fatalf("expected 100 completed files after concurrent updates, got %d", snap.CompletedFiles)
	}
}

func TestConcurrentMixedOperations(t *testing.T) {
	tracker := NewSyncTracker("mixed")
	tracker.SetTotals(30, 30000)
	tracker.SetPhase(PhaseDownloading)

	var wg sync.WaitGroup

	// 10 completing
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			tracker.FileCompleted("/ok"+string(rune('0'+i)), 1000)
		}
	}()

	// 10 failing
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			tracker.FileFailed("/fail"+string(rune('0'+i)), "error")
		}
	}()

	// 10 skipping
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			tracker.FileSkipped()
		}
	}()

	// Concurrent snapshots
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			_ = tracker.Snapshot()
		}
	}()

	wg.Wait()

	snap := tracker.Snapshot()
	if snap.CompletedFiles != 10 {
		t.Fatalf("expected 10 completed, got %d", snap.CompletedFiles)
	}
	if snap.FailedFiles != 10 {
		t.Fatalf("expected 10 failed, got %d", snap.FailedFiles)
	}
	if snap.SkippedFiles != 10 {
		t.Fatalf("expected 10 skipped, got %d", snap.SkippedFiles)
	}
}

func TestSnapshotCurrentFiles(t *testing.T) {
	tracker := NewSyncTracker("test")
	tracker.SetTotals(3, 3000)

	// Add active files directly
	tracker.mu.Lock()
	tracker.files["/active1"] = &FileProgress{Path: "/active1", BytesDownloaded: 100, TotalBytes: 1000}
	tracker.files["/active2"] = &FileProgress{Path: "/active2", BytesDownloaded: 200, TotalBytes: 1000}
	tracker.files["/done1"] = &FileProgress{Path: "/done1", BytesDownloaded: 1000, TotalBytes: 1000, Done: true}
	tracker.mu.Unlock()

	snap := tracker.Snapshot()

	// Only non-done, non-failed files should appear in CurrentFiles
	if len(snap.CurrentFiles) != 2 {
		t.Fatalf("expected 2 current files, got %d", len(snap.CurrentFiles))
	}
	// Should be sorted by path
	if snap.CurrentFiles[0].Path != "/active1" || snap.CurrentFiles[1].Path != "/active2" {
		t.Fatalf("expected sorted current files, got %v", snap.CurrentFiles)
	}
}

func TestSnapshotIsolation(t *testing.T) {
	tracker := NewSyncTracker("test")
	tracker.SetTotals(2, 2000)
	tracker.FileCompleted("/a", 1000)

	snap1 := tracker.Snapshot()

	// Mutate after snapshot
	tracker.FileCompleted("/b", 1000)

	// snap1 should not be affected
	if snap1.CompletedFiles != 1 {
		t.Fatalf("snapshot should be isolated, expected 1 completed, got %d", snap1.CompletedFiles)
	}

	snap2 := tracker.Snapshot()
	if snap2.CompletedFiles != 2 {
		t.Fatalf("expected 2 completed in new snapshot, got %d", snap2.CompletedFiles)
	}
}
