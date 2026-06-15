package torr

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/anacrolix/torrent"

	"server/log"
	sets "server/settings"
	"server/torr/state"
)

type DownloadInfo struct {
	Hash        string
	Path        string
	TotalSize   int64
	Downloaded  int64
	FileCount   int
	FilesDone   int
	ExpiryDate  time.Time
	StartedAt   time.Time
	CompletedAt time.Time
	IsComplete  bool
	IsError     bool
	ErrorMsg    string
	CancelFunc  context.CancelFunc
}

type DownloadManager struct {
	mu        sync.RWMutex
	downloads map[string]*DownloadInfo
	wg        sync.WaitGroup
}

var downloadMgr *DownloadManager

func InitDownloadManager() {
	downloadMgr = &DownloadManager{
		downloads: make(map[string]*DownloadInfo),
	}
	go downloadMgr.cleanupLoop()
	downloadMgr.cleanupOrphanedFiles()
	log.TLogln("Download manager initialized")
}

func GetDownloadManager() *DownloadManager {
	return downloadMgr
}

func (dm *DownloadManager) StartDownload(t *Torrent, ttlMinutes int) error {
	if !sets.BTsets.EnableDownload {
		return fmt.Errorf("download feature is disabled")
	}

	if sets.BTsets.DownloadPath == "" {
		return fmt.Errorf("download path is not configured")
	}

	hash := t.Hash().HexString()

	dm.mu.Lock()
	if _, exists := dm.downloads[hash]; exists {
		dm.mu.Unlock()
		return fmt.Errorf("download already in progress for hash: %s", hash)
	}
	dm.mu.Unlock()

	if t.Torrent == nil || t.Info() == nil {
		return fmt.Errorf("torrent has no info")
	}

	t.muTorrent.Lock()
	if t.Stat != state.TorrentWorking && t.Stat != state.TorrentDownloading {
		t.muTorrent.Unlock()
		return fmt.Errorf("torrent is not in a downloadable state")
	}
	t.Stat = state.TorrentDownloading
	t.muTorrent.Unlock()

	ctx, cancel := context.WithCancel(context.Background())

	downloadPath := filepath.Join(sets.BTsets.DownloadPath, hash)

	info := &DownloadInfo{
		Hash:       hash,
		Path:       downloadPath,
		TotalSize:  t.Length(),
		FileCount:  len(t.Files()),
		StartedAt:  time.Now(),
		CancelFunc: cancel,
	}

	if ttlMinutes > 0 {
		info.ExpiryDate = time.Now().Add(time.Duration(ttlMinutes) * time.Minute)
	}

	dm.mu.Lock()
	dm.downloads[hash] = info
	dm.mu.Unlock()

	dm.wg.Add(1)
	go dm.downloadTorrent(ctx, t, info)

	return nil
}

func (dm *DownloadManager) downloadTorrent(ctx context.Context, t *Torrent, info *DownloadInfo) {
	defer dm.wg.Done()

	var downloadErr error
	defer func() {
		info.CompletedAt = time.Now()

		if downloadErr != nil {
			info.IsError = true
			info.ErrorMsg = downloadErr.Error()
			log.TLogln("Download failed:", info.Hash, downloadErr)
			// Clean up partial files on error
			os.RemoveAll(info.Path)
		} else {
			info.IsComplete = true
			dlDB := &sets.DownloadDB{
				Hash:       info.Hash,
				Path:       info.Path,
				ExpiryDate: info.ExpiryDate.Unix(),
				CreatedAt:  info.StartedAt.Unix(),
				TotalSize:  info.TotalSize,
				FileCount:  info.FileCount,
			}
			sets.AddDownload(dlDB)

			// Mark pieces as downloaded so they are not evicted from cache
			// This allows the torrent to seed the downloaded files
			if t.cache != nil && t.Info() != nil {
				pieceLength := t.Info().PieceLength
				var allPieceIds []int
				seen := make(map[int]bool)
				for _, file := range t.Files() {
					startPiece := int(file.Offset() / pieceLength)
					endPiece := int((file.Offset() + file.Length() - 1) / pieceLength)
					for i := startPiece; i <= endPiece; i++ {
						if !seen[i] {
							allPieceIds = append(allPieceIds, i)
							seen[i] = true
						}
					}
				}
				t.cache.MarkPiecesDownloaded(allPieceIds)
				log.TLogln("Marked", len(allPieceIds), "pieces as downloaded for seeding:", info.Hash)
			}

			log.TLogln("Download completed:", info.Hash)
		}

		t.muTorrent.Lock()
		if t.Stat == state.TorrentDownloading {
			t.Stat = state.TorrentWorking
		}
		t.muTorrent.Unlock()

		t.AddExpiredTime(time.Second * time.Duration(sets.BTsets.TorrentDisconnectTimeout))

		// Remove from active downloads map after completion or error
		// (DB entry persists for cleanup daemon to handle TTL)
		dm.mu.Lock()
		delete(dm.downloads, info.Hash)
		dm.mu.Unlock()
	}()

	// Check if download path is writable before starting
	if err := os.MkdirAll(info.Path, 0755); err != nil {
		downloadErr = fmt.Errorf("cannot create download directory: %w", err)
		return
	}

	// Test write permission
	testFile := filepath.Join(info.Path, ".write_test")
	if f, err := os.Create(testFile); err != nil {
		downloadErr = fmt.Errorf("download path is not writable: %w", err)
		return
	} else {
		f.Close()
		os.Remove(testFile)
	}

	// Check available disk space
	var diskUsage syscall.Statfs_t
	if err := syscall.Statfs(info.Path, &diskUsage); err == nil {
		available := int64(diskUsage.Bavail) * int64(diskUsage.Bsize)
		if available < info.TotalSize {
			downloadErr = fmt.Errorf("insufficient disk space: need %d MB, have %d MB",
				info.TotalSize/1024/1024, available/1024/1024)
			return
		}
	}

	// Increase cache capacity for download to prevent piece eviction
	if t.cache != nil {
		t.cache.IncreaseCapacity(info.TotalSize)
		log.TLogln("Increased cache capacity for download:", info.TotalSize/1024/1024, "MB")
	}

	files := t.Files()
	for i, file := range files {
		select {
		case <-ctx.Done():
			downloadErr = ctx.Err()
			log.TLogln("Download cancelled:", info.Hash)
			return
		default:
		}

		filePath := filepath.Join(info.Path, file.Path())
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			downloadErr = fmt.Errorf("cannot create file directory: %w", err)
			return
		}

		if err := dm.downloadFile(ctx, t, file, filePath, info, i); err != nil {
			downloadErr = err
			return
		}

		info.FilesDone++
	}
}

func (dm *DownloadManager) downloadFile(ctx context.Context, t *Torrent, file *torrent.File, filePath string, info *DownloadInfo, fileIndex int) error {
	reader := t.cache.NewReader(file)
	if reader == nil {
		return fmt.Errorf("cannot create reader for file")
	}
	defer t.CloseReader(reader)

	// Use high readahead for parallel piece downloading
	pieceLength := t.Info().PieceLength
	readahead := pieceLength * 8
	if readahead < 16<<20 {
		readahead = 16 << 20
	}
	reader.SetReadahead(readahead)

	outFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("cannot create file: %w", err)
	}
	defer outFile.Close()

	buf := make([]byte, 32768)
	for {
		select {
		case <-ctx.Done():
			// Clean up partial file on cancel
			outFile.Close()
			os.Remove(filePath)
			return ctx.Err()
		default:
		}

		n, err := reader.Read(buf)
		if n > 0 {
			if _, writeErr := outFile.Write(buf[:n]); writeErr != nil {
				outFile.Close()
				os.Remove(filePath)
				return fmt.Errorf("error writing file: %w", writeErr)
			}
			info.Downloaded += int64(n)
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			outFile.Close()
			os.Remove(filePath)
			return fmt.Errorf("error reading: %w", err)
		}
	}
}

func (dm *DownloadManager) CancelDownload(hash string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	info, exists := dm.downloads[hash]
	if !exists {
		return fmt.Errorf("download not found for hash: %s", hash)
	}

	info.CancelFunc()

	// Wait for the download goroutine to finish
	dm.mu.Unlock()
	dm.wg.Wait()
	dm.mu.Lock()

	os.RemoveAll(info.Path)
	sets.RemDownload(hash)

	delete(dm.downloads, hash)

	return nil
}

func (dm *DownloadManager) GetDownloadStatus(hash string) *DownloadInfo {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.downloads[hash]
}

func (dm *DownloadManager) ListDownloads() []*DownloadInfo {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	var list []*DownloadInfo
	for _, info := range dm.downloads {
		list = append(list, info)
	}

	// Also include completed downloads from DB that are not in memory
	dbDownloads := sets.ListDownloads()
	for _, db := range dbDownloads {
		found := false
		for _, info := range list {
			if info.Hash == db.Hash {
				found = true
				break
			}
		}
		if !found {
			info := &DownloadInfo{
				Hash:       db.Hash,
				Path:       db.Path,
				TotalSize:  db.TotalSize,
				FileCount:  db.FileCount,
				FilesDone:  db.FileCount,
				IsComplete: true,
				ExpiryDate: time.Unix(db.ExpiryDate, 0),
				StartedAt:  time.Unix(db.CreatedAt, 0),
			}
			list = append(list, info)
		}
	}

	return list
}

func (dm *DownloadManager) cleanupLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		dm.cleanupExpired()
	}
}

// cleanupOrphanedFiles removes download directories that are not tracked in DB
// This handles cases where server crashed during download
func (dm *DownloadManager) cleanupOrphanedFiles() {
	if sets.ReadOnly || sets.BTsets.DownloadPath == "" {
		return
	}

	dirs, err := os.ReadDir(sets.BTsets.DownloadPath)
	if err != nil {
		return
	}

	dbDownloads := sets.ListDownloads()
	dbHashes := make(map[string]bool)
	for _, dl := range dbDownloads {
		dbHashes[dl.Hash] = true
	}

	for _, dir := range dirs {
		if dir.IsDir() && len(dir.Name()) == 40 { // infohash is 40 hex chars
			if !dbHashes[dir.Name()] {
				log.TLogln("Removing orphaned download directory:", dir.Name())
				os.RemoveAll(filepath.Join(sets.BTsets.DownloadPath, dir.Name()))
			}
		}
	}
}

func (dm *DownloadManager) cleanupExpired() {
	if sets.ReadOnly {
		return
	}

	dbDownloads := sets.ListDownloads()
	now := time.Now()

	for _, dl := range dbDownloads {
		if dl.ExpiryDate > 0 && dl.ExpiryDate < now.Unix() {
			// Skip if download is still active
			dm.mu.RLock()
			_, isActive := dm.downloads[dl.Hash]
			dm.mu.RUnlock()
			if isActive {
				log.TLogln("Skipping cleanup for active download:", dl.Hash)
				continue
			}

			log.TLogln("Removing expired download:", dl.Hash)

			os.RemoveAll(dl.Path)
			sets.RemDownload(dl.Hash)

			dm.mu.Lock()
			delete(dm.downloads, dl.Hash)
			dm.mu.Unlock()
		}
	}
}

func GetDownloadInfoByHash(hash string) *sets.DownloadDB {
	return sets.GetDownload(hash)
}

func IsFileDownloaded(hash, filePath string) bool {
	if !sets.BTsets.EnableDownload || sets.BTsets.DownloadPath == "" {
		return false
	}

	dl := sets.GetDownload(hash)
	if dl == nil {
		return false
	}

	fullPath := filepath.Join(dl.Path, filePath)
	_, err := os.Stat(fullPath)
	return err == nil
}

func GetDownloadFilePath(hash, filePath string) string {
	if !sets.BTsets.EnableDownload || sets.BTsets.DownloadPath == "" {
		return ""
	}

	dl := sets.GetDownload(hash)
	if dl == nil {
		return ""
	}

	fullPath := filepath.Join(dl.Path, filePath)
	if _, err := os.Stat(fullPath); err != nil {
		return ""
	}
	return fullPath
}
