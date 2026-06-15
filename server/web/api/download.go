package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"server/torr"
)

type downloadReqJS struct {
	requestI
	Hash string `json:"hash,omitempty"`
	TTL  int    `json:"ttl,omitempty"` // minutes, 0 = no expiry
}

func download(c *gin.Context) {
	var req downloadReqJS
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	switch req.Action {
	case "start":
		startDownload(c, req)
	case "cancel":
		cancelDownload(c, req)
	case "status":
		getDownloadStatus(c, req)
	case "list":
		listDownloads(c)
	default:
		c.AbortWithError(http.StatusBadRequest, http.ErrNoLocation)
	}
}

func startDownload(c *gin.Context, req downloadReqJS) {
	if req.Hash == "" {
		c.AbortWithError(http.StatusBadRequest, http.ErrNoLocation)
		return
	}

	tor := torr.GetTorrent(req.Hash)
	if tor == nil {
		c.AbortWithError(http.StatusNotFound, http.ErrNoLocation)
		return
	}

	ttl := req.TTL
	if ttl == 0 {
		ttl = 30 * 24 * 60 // 30 days default
	}

	dlMgr := torr.GetDownloadManager()
	if err := dlMgr.StartDownload(tor, ttl); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"hash":   req.Hash,
		"ttl":    ttl,
		"status": "started",
	})
}

func cancelDownload(c *gin.Context, req downloadReqJS) {
	if req.Hash == "" {
		c.AbortWithError(http.StatusBadRequest, http.ErrNoLocation)
		return
	}

	dlMgr := torr.GetDownloadManager()
	if err := dlMgr.CancelDownload(req.Hash); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"hash":   req.Hash,
		"status": "cancelled",
	})
}

func getDownloadStatus(c *gin.Context, req downloadReqJS) {
	if req.Hash == "" {
		c.AbortWithError(http.StatusBadRequest, http.ErrNoLocation)
		return
	}

	dlMgr := torr.GetDownloadManager()
	info := dlMgr.GetDownloadStatus(req.Hash)
	if info == nil {
		dlDB := torr.GetDownloadInfoByHash(req.Hash)
		if dlDB == nil {
			c.AbortWithError(http.StatusNotFound, http.ErrNoLocation)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"hash":        dlDB.Hash,
			"path":        dlDB.Path,
			"total_size":  dlDB.TotalSize,
			"file_count":  dlDB.FileCount,
			"expiry_date": dlDB.ExpiryDate,
			"created_at":  dlDB.CreatedAt,
			"status":      "completed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"hash":        info.Hash,
		"path":        info.Path,
		"total_size":  info.TotalSize,
		"downloaded":  info.Downloaded,
		"file_count":  info.FileCount,
		"files_done":  info.FilesDone,
		"expiry_date": info.ExpiryDate.Unix(),
		"is_complete":  info.IsComplete,
		"status":      "downloading",
	})
}

func listDownloads(c *gin.Context) {
	dlMgr := torr.GetDownloadManager()
	downloads := dlMgr.ListDownloads()

	var result []gin.H
	for _, dl := range downloads {
		item := gin.H{
			"hash":        dl.Hash,
			"path":        dl.Path,
			"total_size":  dl.TotalSize,
			"downloaded":  dl.Downloaded,
			"file_count":  dl.FileCount,
			"files_done":  dl.FilesDone,
			"expiry_date": dl.ExpiryDate.Unix(),
			"is_complete":  dl.IsComplete,
		}
		if dl.IsError {
			item["error"] = dl.ErrorMsg
		}
		result = append(result, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"downloads": result,
	})
}
