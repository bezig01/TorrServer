package settings

import (
	"testing"
)

func TestBTSetsDownloadFields(t *testing.T) {
	sets := &BTSets{
		EnableDownload: true,
		DownloadPath:   "/tmp/downloads",
		DownloadTTL:    43200, // 30 days in minutes
	}

	if !sets.EnableDownload {
		t.Error("EnableDownload should be true")
	}
	if sets.DownloadPath != "/tmp/downloads" {
		t.Errorf("DownloadPath = %s, want /tmp/downloads", sets.DownloadPath)
	}
	if sets.DownloadTTL != 43200 {
		t.Errorf("DownloadTTL = %d, want 43200", sets.DownloadTTL)
	}
}

func TestBTSetsDownloadMarshal(t *testing.T) {
	sets := &BTSets{
		EnableDownload: true,
		DownloadPath:   "/tmp/downloads",
		DownloadTTL:    43200,
	}

	data := sets.String()
	if data == "" {
		t.Error("String() returned empty")
	}
}
