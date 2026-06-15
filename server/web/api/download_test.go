package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestDownloadHandlerStartMissingHash(t *testing.T) {
	router := setupTestRouter()
	router.POST("/download", download)

	reqBody := downloadReqJS{
		requestI: requestI{Action: "start"},
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/download", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestDownloadHandlerCancelMissingHash(t *testing.T) {
	router := setupTestRouter()
	router.POST("/download", download)

	reqBody := downloadReqJS{
		requestI: requestI{Action: "cancel"},
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/download", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestDownloadHandlerStatusMissingHash(t *testing.T) {
	router := setupTestRouter()
	router.POST("/download", download)

	reqBody := downloadReqJS{
		requestI: requestI{Action: "status"},
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/download", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestDownloadHandlerInvalidAction(t *testing.T) {
	router := setupTestRouter()
	router.POST("/download", download)

	reqBody := downloadReqJS{
		requestI: requestI{Action: "invalid"},
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/download", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}
