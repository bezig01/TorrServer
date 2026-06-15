package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"server/torr"
)

// downloadReqJS represents the download request body
type downloadReqJS struct {
	requestI
	Hash string `json:"hash,omitempty"`
	TTL  int    `json:"ttl,omitempty"` // minutes, 0 = no expiry
}

// download godoc
//
//	@Summary		Handle download operations
//	@Description	Download, cancel, get status, or list downloaded torrents
//
//	@Tags			API
//
//	@Param			request	body	downloadReqJS	true	"Download request. Available actions: start (requires hash, optional ttl), cancel (requires hash), status (requires hash), list"
//
//	@Accept			json
//	@Produce		json
//	@Success		200
//	@Router			/download [post]
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

// startDownload godoc
//
//	@Summary		Start downloading a torrent
//	@Description	Begin downloading all files of a torrent to disk for offline viewing
//
//	@Tags			API
//
//	@Param			request	body	downloadReqJS	true	"Download start request"
//
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	object	"Download started"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/download [post]
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

// cancelDownload godoc
//
//	@Summary		Cancel a download
//	@Description	Cancel an active download and remove partial files
//
//	@Tags			API
//
//	@Param			request	body	downloadReqJS	true	"Download cancel request"
//
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	object	"Download cancelled"
//	@Failure		400
//	@Failure		500
//	@Router			/download [post]
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

// getDownloadStatus godoc
//
//	@Summary		Get download status
//	@Description	Get current status of a download including progress
//
//	@Tags			API
//
//	@Param			request	body	downloadReqJS	true	"Download status request"
//
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	object	"Download status"
//	@Failure		400
//	@Failure		404
//	@Router			/download [post]
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

// listDownloads godoc
//
//	@Summary		List all downloads
//	@Description	List all active and completed downloads
//
//	@Tags			API
//
//	@Param			request	body	downloadReqJS	true	"Download list request"
//
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	object	"List of downloads"
//	@Router			/download [post]
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
