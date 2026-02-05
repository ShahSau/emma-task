package routers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gothinkster/golang-gin-realworld-example-app/common"
	"github.com/gothinkster/golang-gin-realworld-example-app/jobs"
	"github.com/gothinkster/golang-gin-realworld-example-app/jobs/core"
)

type ExportRequest struct {
	Resource string            `json:"resource" binding:"required"`
	Format   string            `json:"format"`
	Filters  map[string]string `json:"filters"`
}

type ExportConfig struct {
	Format  string            `json:"format"`
	Filters map[string]string `json:"filters"`
}

// AsyncExport (POST /v1/exports)
func AsyncExport(c *gin.Context) {
	var req ExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Resource != "users" && req.Resource != "articles" && req.Resource != "comments" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid resource"})
		return
	}
	if req.Format == "" {
		req.Format = "ndjson"
	}

	if req.Format != "ndjson" && req.Format != "csv" && req.Format != "json" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid format. Use: ndjson, csv, or json"})
		return
	}

	jobUUID := uuid.New()

	// Pack configuration into JSON for the SourceKey
	config := ExportConfig{
		Format:  req.Format,
		Filters: req.Filters,
	}
	configBytes, _ := json.Marshal(config)

	job := jobs.Job{
		ID:             jobUUID,
		Type:           jobs.TypeExport,
		Resource:       req.Resource,
		Status:         jobs.StatusPending,
		SourceKey:      string(configBytes),
		IdempotencyKey: jobUUID.String(),
	}

	db := common.GetDB()
	if err := db.Create(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create job"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"job_id":  job.ID.String(),
		"status":  "PENDING",
		"message": "Export job started",
	})
}

// SyncExport (GET /v1/exports)
func SyncExport(c *gin.Context) {
	resource := c.Query("resource")
	format := c.Query("format")

	// Collect filters from query params
	filters := make(map[string]string)
	if author := c.Query("author"); author != "" {
		filters["author"] = author
	}
	if tag := c.Query("tag"); tag != "" {
		filters["tag"] = tag
	}
	if username := c.Query("username"); username != "" {
		filters["username"] = username
	}
	if article := c.Query("article"); article != "" {
		filters["article"] = article
	}

	if resource != "users" && resource != "articles" && resource != "comments" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid resource"})
		return
	}
	if format == "" {
		format = "ndjson"
	}

	contentType := "application/x-ndjson"
	if format == "csv" {
		contentType = "text/csv"
	}
	if format == "json" {
		contentType = "application/json"
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.%s", resource, format))
	c.Header("Content-Type", contentType)
	c.Header("Transfer-Encoding", "chunked")

	_, err := core.StreamExport(resource, format, filters, c.Writer)
	if err != nil {
		fmt.Printf("Stream error: %v\n", err)
	}
}
