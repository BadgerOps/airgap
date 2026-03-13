package engine

import (
	"testing"
)

func TestCollectRPMRepoDirsNoRPMProviders(t *testing.T) {
	manifest := &TransferManifest{
		Providers: map[string]ManifestProvider{
			"ocp-binaries": {Type: "generic", FileCount: 5},
		},
		FileInventory: []ManifestFile{
			{Provider: "ocp-binaries", Path: "bin/oc", Size: 1024},
		},
	}

	dirs := collectRPMRepoDirs(manifest, "/data")
	if len(dirs) != 0 {
		t.Fatalf("expected no RPM repo dirs for non-rpm provider, got %d", len(dirs))
	}
}

func TestCollectRPMRepoDirsWithRPMProvider(t *testing.T) {
	manifest := &TransferManifest{
		Providers: map[string]ManifestProvider{
			"epel": {Type: "rpm_repo", FileCount: 3},
		},
		FileInventory: []ManifestFile{
			{Provider: "epel", Path: "9/Packages/a.rpm", Size: 1024},
			{Provider: "epel", Path: "9/Packages/b.rpm", Size: 2048},
			{Provider: "epel", Path: "8/Packages/c.rpm", Size: 512},
		},
	}

	dirs := collectRPMRepoDirs(manifest, "/data")
	if len(dirs) != 2 {
		t.Fatalf("expected 2 unique RPM repo dirs, got %d: %v", len(dirs), dirs)
	}
}

func TestCollectRPMRepoDirsDeduplication(t *testing.T) {
	manifest := &TransferManifest{
		Providers: map[string]ManifestProvider{
			"epel": {Type: "rpm_repo", FileCount: 3},
		},
		FileInventory: []ManifestFile{
			{Provider: "epel", Path: "9/Packages/a.rpm"},
			{Provider: "epel", Path: "9/Packages/b.rpm"},
			{Provider: "epel", Path: "9/repodata/repomd.xml"},
		},
	}

	dirs := collectRPMRepoDirs(manifest, "/data")
	if len(dirs) != 1 {
		t.Fatalf("expected 1 unique RPM repo dir (all under '9'), got %d: %v", len(dirs), dirs)
	}
}

func TestCollectRPMRepoDirsMixedProviders(t *testing.T) {
	manifest := &TransferManifest{
		Providers: map[string]ManifestProvider{
			"epel":         {Type: "rpm_repo", FileCount: 2},
			"ocp-binaries": {Type: "generic", FileCount: 1},
		},
		FileInventory: []ManifestFile{
			{Provider: "epel", Path: "9/Packages/a.rpm"},
			{Provider: "ocp-binaries", Path: "bin/oc"},
			{Provider: "epel", Path: "8/Packages/b.rpm"},
		},
	}

	dirs := collectRPMRepoDirs(manifest, "/data")
	if len(dirs) != 2 {
		t.Fatalf("expected 2 RPM repo dirs (only from epel), got %d: %v", len(dirs), dirs)
	}
}

func TestCollectRPMRepoDirsTraversalPath(t *testing.T) {
	manifest := &TransferManifest{
		Providers: map[string]ManifestProvider{
			"epel": {Type: "rpm_repo", FileCount: 1},
		},
		FileInventory: []ManifestFile{
			{Provider: "epel", Path: "../escape/file.rpm"},
		},
	}

	dirs := collectRPMRepoDirs(manifest, "/data")
	// Traversal path should be skipped by safety.CleanRelativePath
	if len(dirs) != 0 {
		t.Fatalf("expected traversal path to be skipped, got %d dirs: %v", len(dirs), dirs)
	}
}

func TestCollectRPMRepoDirsEmptyManifest(t *testing.T) {
	manifest := &TransferManifest{
		Providers:     map[string]ManifestProvider{},
		FileInventory: []ManifestFile{},
	}

	dirs := collectRPMRepoDirs(manifest, "/data")
	if len(dirs) != 0 {
		t.Fatalf("expected no dirs for empty manifest, got %d", len(dirs))
	}
}
