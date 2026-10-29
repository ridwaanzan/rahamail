package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mail-server/internal/backup"
	"mail-server/internal/repository"

	"github.com/gin-gonic/gin"
)

func TestPhase3Flow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	repository.InitDB()
	mailCtrl := NewMailController()

	api := r.Group("/api/v1")
	{
		api.GET("/admin/backup/config", mailCtrl.GetBackupConfig)
		api.POST("/admin/backup/config", mailCtrl.SaveBackupConfig)
		api.POST("/admin/backup/trigger", mailCtrl.TriggerManualBackup)
	}

	// 1. Test Save Backup Config (tanpa remote S3 aktif untuk testing lokal)
	s3Payload, _ := json.Marshal(backup.S3Config{
		Bucket:        "my-mail-backups",
		EncryptionKey: "super-secret-key-123",
		RetentionDays: 60,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/admin/backup/config", bytes.NewBuffer(s3Payload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for save backup config, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Test Get Backup Config
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/admin/backup/config", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for get backup config, got %d", w.Code)
	}
	var loadedCfg backup.S3Config
	json.Unmarshal(w.Body.Bytes(), &loadedCfg)
	if loadedCfg.Bucket != "my-mail-backups" {
		t.Fatalf("Expected bucket my-mail-backups, got %s", loadedCfg.Bucket)
	}

	// 3. Test Trigger Manual Snapshot Backup
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/admin/backup/trigger", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for manual backup trigger, got %d: %s", w.Code, w.Body.String())
	}

	var backupResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &backupResp)
	if backupResp["status"] != "success" {
		t.Fatalf("Expected success backup status, got %v", backupResp["status"])
	}
}
