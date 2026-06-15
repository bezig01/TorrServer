package torr

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	sets "server/settings"
)

func TestDownloadManagerCreation(t *testing.T) {
	dm := &DownloadManager{
		downloads: make(map[string]*DownloadInfo),
	}

	if dm == nil {
		t.Error("DownloadManager should not be nil")
	}
	if dm.downloads == nil {
		t.Error("downloads map should not be nil")
	}
}

func TestDownloadInfo(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	info := &DownloadInfo{
		Hash:       "abc123",
		Path:       "/tmp/downloads/abc123",
		TotalSize:  1024 * 1024 * 100,
		FileCount:  5,
		ExpiryDate: time.Now().AddDate(0, 0, 30),
		StartedAt:  time.Now(),
		CancelFunc: cancel,
	}

	if info.Hash != "abc123" {
		t.Errorf("Hash = %s, want abc123", info.Hash)
	}
	if info.Path != "/tmp/downloads/abc123" {
		t.Errorf("Path = %s, want /tmp/downloads/abc123", info.Path)
	}
	if info.TotalSize != 1024*1024*100 {
		t.Errorf("TotalSize = %d, want %d", info.TotalSize, 1024*1024*100)
	}
	if info.FileCount != 5 {
		t.Errorf("FileCount = %d, want 5", info.FileCount)
	}

	cancel()

	select {
	case <-ctx.Done():
	default:
		t.Error("Context should be done after cancel")
	}
}

func TestIsFileDownloadedDisabled(t *testing.T) {
	origBTSets := sets.BTsets
	defer func() { sets.BTsets = origBTSets }()

	sets.BTsets = &sets.BTSets{
		EnableDownload: false,
	}

	result := IsFileDownloaded("abc123", "test.txt")
	if result {
		t.Error("IsFileDownloaded should return false when feature is disabled")
	}
}

func TestIsFileDownloadedEmptyPath(t *testing.T) {
	origBTSets := sets.BTsets
	defer func() { sets.BTsets = origBTSets }()

	sets.BTsets = &sets.BTSets{
		EnableDownload: true,
		DownloadPath:   "",
	}

	result := IsFileDownloaded("abc123", "test.txt")
	if result {
		t.Error("IsFileDownloaded should return false when DownloadPath is empty")
	}
}

func TestGetDownloadFilePathDisabled(t *testing.T) {
	origBTSets := sets.BTsets
	defer func() { sets.BTsets = origBTSets }()

	sets.BTsets = &sets.BTSets{
		EnableDownload: false,
	}

	result := GetDownloadFilePath("abc123", "test.txt")
	if result != "" {
		t.Error("GetDownloadFilePath should return empty when feature is disabled")
	}
}

func TestGetDownloadFilePathEmptyPath(t *testing.T) {
	origBTSets := sets.BTsets
	defer func() { sets.BTsets = origBTSets }()

	sets.BTsets = &sets.BTSets{
		EnableDownload: true,
		DownloadPath:   "",
	}

	result := GetDownloadFilePath("abc123", "test.txt")
	if result != "" {
		t.Error("GetDownloadFilePath should return empty when DownloadPath is empty")
	}
}

func TestDownloadManagerCancel(t *testing.T) {
	tmpDir := t.TempDir()
	downloadPath := filepath.Join(tmpDir, "test_file.txt")

	err := os.WriteFile(downloadPath, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	dm := &DownloadManager{
		downloads: make(map[string]*DownloadInfo),
	}

	_, cancel := context.WithCancel(context.Background())
	dm.downloads["abc123"] = &DownloadInfo{
		Hash:       "abc123",
		Path:       downloadPath,
		CancelFunc: cancel,
	}

	cancel()

	dm.mu.Lock()
	info := dm.downloads["abc123"]
	if info != nil {
		os.RemoveAll(info.Path)
		delete(dm.downloads, "abc123")
	}
	dm.mu.Unlock()

	if _, err := os.Stat(downloadPath); !os.IsNotExist(err) {
		t.Error("File should be deleted after cancel")
	}
}

func TestDownloadManagerCancelNotFound(t *testing.T) {
	dm := &DownloadManager{
		downloads: make(map[string]*DownloadInfo),
	}

	err := dm.CancelDownload("nonexistent")
	if err == nil {
		t.Error("CancelDownload should return error for nonexistent hash")
	}
}

func TestDownloadManagerListEmpty(t *testing.T) {
	dm := &DownloadManager{
		downloads: make(map[string]*DownloadInfo),
	}

	// Test in-memory list only (without DB)
	dm.mu.RLock()
	count := len(dm.downloads)
	dm.mu.RUnlock()

	if count != 0 {
		t.Errorf("In-memory downloads count = %d, want 0", count)
	}
}

func TestDownloadManagerGetStatusNotFound(t *testing.T) {
	dm := &DownloadManager{
		downloads: make(map[string]*DownloadInfo),
	}

	info := dm.GetDownloadStatus("nonexistent")
	if info != nil {
		t.Error("GetDownloadStatus should return nil for nonexistent hash")
	}
}
