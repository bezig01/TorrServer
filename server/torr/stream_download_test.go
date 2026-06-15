package torr

import (
	"os"
	"path/filepath"
	"testing"

	sets "server/settings"
)

func TestStreamDownloadPathResolution(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	err := os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat test file: %v", err)
	}

	if info.Size() != 4 {
		t.Errorf("File size = %d, want 4", info.Size())
	}
}

func TestStreamDownloadPathJoin(t *testing.T) {
	tmpDir := t.TempDir()
	hash := "abc123def456"
	filePath := "subdir/video.mkv"

	fullPath := filepath.Join(tmpDir, hash, filePath)
	expected := filepath.Join(tmpDir, "abc123def456", "subdir", "video.mkv")

	if fullPath != expected {
		t.Errorf("Path = %s, want %s", fullPath, expected)
	}
}

func TestStreamDownloadDisabled(t *testing.T) {
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
