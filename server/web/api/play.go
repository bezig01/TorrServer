package api

import (
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/anacrolix/missinggo/v2/httptoo"
	"github.com/gin-gonic/gin"

	mt "server/mimetype"
	sets "server/settings"
	"server/torr"
	"server/torr/state"
	"server/web/api/utils"
)

// play godoc
//
//	@Summary		Play given torrent by infohash
//	@Description	Play given torrent referenced by infohash and file id.
//
//	@Tags			API
//
//	@Param			hash		path	string	true	"Torrent infohash"
//	@Param			id			path	string	true	"File index in torrent"
//
//	@Produce		application/octet-stream
//	@Success		200	"Torrent data"
//	@Router			/play/{hash}/{id} [get]
func play(c *gin.Context) {
	hash := c.Param("hash")
	indexStr := c.Param("id")
	notAuth := c.GetBool("auth_required") && c.GetString(gin.AuthUserKey) == ""

	if hash == "" || indexStr == "" {
		c.AbortWithError(http.StatusNotFound, errors.New("no infohash or file index in link"))
		return
	}

	spec, err := utils.ParseLink(hash)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	tor := torr.GetTorrent(spec.InfoHash.HexString())
	if tor == nil && notAuth {
		c.Header("WWW-Authenticate", "Basic realm=Authorization Required")
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	if tor == nil {
		c.AbortWithError(http.StatusInternalServerError, errors.New("error get torrent"))
		return
	}

	if tor.Stat == state.TorrentInDB {
		tor, err = torr.AddTorrent(spec, tor.Title, tor.Poster, tor.Data, tor.Category)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
	}

	// Check for downloaded file — try DB first (works even without torrent connection)
	dlHash := spec.InfoHash.HexString()
	ind, _ := strconv.Atoi(indexStr)
	if sets.BTsets.EnableDownload && sets.BTsets.DownloadPath != "" {
		// Try to find file path from DB or filesystem
		if filePath := findDownloadedFile(dlHash, ind, tor); filePath != "" {
			fileName := filepath.Base(filePath)
			serveLocalFile(c, filePath, fileName, dlHash)
			return
		}
	}

	if !tor.GotInfo() {
		c.AbortWithError(http.StatusInternalServerError, errors.New("torrent connection timeout"))
		return
	}

	hash = tor.Hash().HexString()

	// Re-check for downloaded file after GotInfo() (hash might differ)
	if sets.BTsets.EnableDownload && sets.BTsets.DownloadPath != "" {
		if filePath := findDownloadedFile(hash, ind, tor); filePath != "" {
			fileName := filepath.Base(filePath)
			serveLocalFile(c, filePath, fileName, hash)
			return
		}
	}

	// find file
	index := -1
	if len(tor.Files()) == 1 {
		index = 1
	} else {
		ind, err := strconv.Atoi(indexStr)
		if err == nil {
			index = ind
		}
	}
	if index == -1 {
		c.AbortWithError(http.StatusBadRequest, errors.New("file \"index\" is wrong"))
		return
	}

	tor.Stream(index, c.Request, c.Writer)
}

// findDownloadedFile tries to find a downloaded file by hash and file index
// Works even when torrent is not connected (uses DB or filesystem)
func findDownloadedFile(hash string, fileIndex int, tor *torr.Torrent) string {
	// Method 1: Check DB for download record
	dl := sets.GetDownload(hash)
	if dl != nil {
		// We have DB record, but need file path from torrent info
		if tor.Torrent != nil && tor.Info() != nil {
			st := tor.Status()
			for _, fileStat := range st.FileStats {
				if fileStat.Id == fileIndex {
					filePath := filepath.Join(dl.Path, fileStat.Path)
					if _, err := os.Stat(filePath); err == nil {
						return filePath
					}
					break
				}
			}
		}
	}

	// Method 2: Check filesystem directly (scan for any media files)
	if sets.BTsets.DownloadPath != "" {
		downloadDir := filepath.Join(sets.BTsets.DownloadPath, hash)
		if _, err := os.Stat(downloadDir); err == nil {
			// Directory exists, find the file
			if tor.Torrent != nil && tor.Info() != nil {
				st := tor.Status()
				for _, fileStat := range st.FileStats {
					if fileStat.Id == fileIndex {
						filePath := filepath.Join(downloadDir, fileStat.Path)
						if _, err := os.Stat(filePath); err == nil {
							return filePath
						}
						break
					}
				}
			}
		}
	}

	return ""
}

func serveLocalFile(c *gin.Context, filePath, fileName, hash string) {
	file, err := os.Open(filePath)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	resp := c.Writer
	resp.Header().Set("Connection", "close")
	resp.Header().Set("Server", "TorrServer (Portable SDK for UPnP devices)")

	etag := hex.EncodeToString([]byte(hash + "/" + fileName))
	resp.Header().Set("ETag", httptoo.EncodeQuotedString(etag))
	resp.Header().Set("transferMode.dlna.org", "Streaming")

	mime, err := mt.MimeTypeByPath(fileName)
	if err == nil && mime.IsMedia() {
		resp.Header().Set("content-type", mime.String())
	}

	if c.Request.Header.Get("getContentFeatures.dlna.org") != "" {
		resp.Header().Set("contentFeatures.dlna.org", "DLNA.ORG_PN=AVC_MP4_BL_L31_HD_AAC;DLNA.ORG_FLAGS=01700000000000000000000000000000")
	}

	if c.Request.Header.Get("Range") != "" {
		resp.Header().Set("Accept-Ranges", "bytes")
	}

	http.ServeContent(resp, c.Request, fileName, stat.ModTime(), file)
}
