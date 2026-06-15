package settings

import (
	"testing"
	"time"
)

func TestDownloadDB(t *testing.T) {
	dl := &DownloadDB{
		Hash:       "abc123",
		Path:       "/tmp/downloads/abc123",
		ExpiryDate: time.Now().AddDate(0, 0, 30).Unix(),
		CreatedAt:  time.Now().Unix(),
		TotalSize:  1024 * 1024 * 100,
		FileCount:  5,
	}

	if dl.Hash != "abc123" {
		t.Errorf("Hash = %s, want abc123", dl.Hash)
	}
	if dl.Path != "/tmp/downloads/abc123" {
		t.Errorf("Path = %s, want /tmp/downloads/abc123", dl.Path)
	}
	if dl.ExpiryDate == 0 {
		t.Error("ExpiryDate should not be 0")
	}
	if dl.CreatedAt == 0 {
		t.Error("CreatedAt should not be 0")
	}
	if dl.TotalSize != 1024*1024*100 {
		t.Errorf("TotalSize = %d, want %d", dl.TotalSize, 1024*1024*100)
	}
	if dl.FileCount != 5 {
		t.Errorf("FileCount = %d, want 5", dl.FileCount)
	}
}
