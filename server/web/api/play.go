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

	// Check for downloaded file BEFORE GotInfo() so offline files work without peers
	dlHash := spec.InfoHash.HexString()
	if sets.BTsets.EnableDownload && sets.BTsets.DownloadPath != "" {
		if tor.Torrent != nil && tor.Info() != nil {
			st := tor.Status()
			ind, _ := strconv.Atoi(indexStr)
			for _, fileStat := range st.FileStats {
				if fileStat.Id == ind {
					filePath := filepath.Join(sets.BTsets.DownloadPath, dlHash, fileStat.Path)
					if _, err := os.Stat(filePath); err == nil {
						serveLocalFile(c, filePath, fileStat.Path, dlHash)
						return
					}
					break
				}
			}
		}
	}

	if !tor.GotInfo() {
		c.AbortWithError(http.StatusInternalServerError, errors.New("torrent connection timeout"))
		return
	}

	hash = tor.Hash().HexString()

	// Re-check for downloaded file after GotInfo() (hash might differ)
	if sets.BTsets.EnableDownload && sets.BTsets.DownloadPath != "" {
		st := tor.Status()
		ind, _ := strconv.Atoi(indexStr)
		for _, fileStat := range st.FileStats {
			if fileStat.Id == ind {
				filePath := filepath.Join(sets.BTsets.DownloadPath, hash, fileStat.Path)
				if _, err := os.Stat(filePath); err == nil {
					serveLocalFile(c, filePath, fileStat.Path, hash)
					return
				}
				break
			}
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
