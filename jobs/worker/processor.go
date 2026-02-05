package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gothinkster/golang-gin-realworld-example-app/articles"
	"github.com/gothinkster/golang-gin-realworld-example-app/common"
	"github.com/gothinkster/golang-gin-realworld-example-app/jobs"
	"github.com/gothinkster/golang-gin-realworld-example-app/jobs/core"
	"github.com/gothinkster/golang-gin-realworld-example-app/users"
)

// executeJob is the main entry point for the worker to process a job
func executeJob(job *jobs.Job) {
	db := common.GetDB()
	var err error

	// Route based on Job Type
	switch job.Type {
	case jobs.TypeImport:
		err = core.ProcessImport(job)
	case jobs.TypeExport:
		err = processExport(job)
	default:
		err = fmt.Errorf("unknown job type: %s", job.Type)
	}

	// Update Final Status
	if err != nil {
		log.Printf("[Worker] Job %s FAILED: %v", job.ID, err)
		job.Status = jobs.StatusFailed
		job.ErrorMessage = err.Error()
	} else {
		log.Printf("[Worker] Job %s COMPLETED", job.ID)
		job.Status = jobs.StatusCompleted

		// For exports, ensure the final TotalRows matches exactly what was processed
		// (This overwrites the estimate we made at the start of processExport)
		if job.Type == jobs.TypeExport {
			job.TotalRows = job.ProcessedRows
		}
	}

	job.UpdatedAt = time.Now()
	db.Save(job)
}

type ExportConfig struct {
	Format  string            `json:"format"`
	Filters map[string]string `json:"filters"`
}

// processExport handles the export logic: DB -> Stream -> S3 (Zero Disk Usage)
func processExport(job *jobs.Job) error {
	// Parse the SourceKey (contains JSON config)
	var config ExportConfig

	// Try parsing JSON first
	if err := json.Unmarshal([]byte(job.SourceKey), &config); err != nil {
		// Fallback for old jobs or manual tests
		parts := strings.Split(job.SourceKey, "|")
		config.Format = "ndjson"
		if len(parts) > 0 && parts[0] != "" {
			config.Format = parts[0]
		}
		config.Filters = make(map[string]string)
	}

	// Default format
	if config.Format == "" {
		config.Format = "ndjson"
	}

	// Validate format
	if config.Format != "ndjson" && config.Format != "csv" && config.Format != "json" {
		return fmt.Errorf("invalid format: %s (must be ndjson, csv, or json)", config.Format)
	}

	log.Printf("[Worker] ✓ Export started: Job %s, Resource: %s, Format: %s, Filters: %v",
		job.ID, job.Resource, config.Format, config.Filters)

	// ---------------------------------------------------------
	// 1. Estimate Total Rows (Update DB for Progress Tracking)
	// ---------------------------------------------------------
	db := common.GetDB()
	var totalCount int64

	switch job.Resource {
	case "users":
		countQuery := db.Model(&users.UserModel{})
		if username, ok := config.Filters["username"]; ok && username != "" {
			countQuery = countQuery.Where("username = ?", username)
		}
		countQuery.Count(&totalCount)
	case "articles":
		countQuery := db.Model(&articles.ArticleModel{})
		if author, ok := config.Filters["author"]; ok && author != "" {
			var user users.UserModel
			if err := db.Where("username = ?", author).First(&user).Error; err == nil {
				countQuery = countQuery.Where("author_id = ?", user.ID)
			}
		}
		if slug, ok := config.Filters["slug"]; ok && slug != "" {
			countQuery = countQuery.Where("slug = ?", slug)
		}
		countQuery.Count(&totalCount)
	case "comments":
		countQuery := db.Model(&articles.CommentModel{})
		if articleSlug, ok := config.Filters["article"]; ok && articleSlug != "" {
			var art articles.ArticleModel
			if err := db.Where("slug = ?", articleSlug).First(&art).Error; err == nil {
				countQuery = countQuery.Where("article_id = ?", art.ID)
			}
		}
		countQuery.Count(&totalCount)
	}

	job.TotalRows = int(totalCount)
	db.Save(job) // Update job with estimated total immediately
	log.Printf("[Worker] ✓ Estimated %d total rows to export", totalCount)

	// ---------------------------------------------------------
	// 2. Setup Pipe for Zero-Copy Streaming
	// ---------------------------------------------------------
	// pr (Reader) goes to S3, pw (Writer) goes to the Exporter
	pr, pw := io.Pipe()

	var exportErr error
	var rowCount int

	// Goroutine: Stream from DB to Pipe
	go func() {
		defer pw.Close() // Close writer when done so S3 knows stream ended

		rows, err := core.StreamExport(job.Resource, config.Format, config.Filters, pw)
		if err != nil {
			exportErr = err
			// Close with error so the S3 reader knows something went wrong
			pw.CloseWithError(err)
			log.Printf("[Worker] ✗ Export streaming failed: %v", err)
			return
		}

		rowCount = rows
		log.Printf("[Worker] ✓ Streaming complete: %d rows written to pipe", rows)
	}()

	// ---------------------------------------------------------
	// 3. Upload Stream Directly to S3
	// ---------------------------------------------------------
	key := fmt.Sprintf("exports/%s/%s-%s.%s", job.Resource, job.Resource, job.ID, config.Format)

	startUpload := time.Now()
	_, err := common.GetS3().PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(os.Getenv("S3_BUCKET")),
		Key:    aws.String(key),
		Body:   pr, // Stream directly from pipe (No temp file created!)
	})
	uploadDuration := time.Since(startUpload)

	// Check for errors from the streamer goroutine
	if exportErr != nil {
		return fmt.Errorf("export streaming error: %v", exportErr)
	}

	// Check for errors from S3 upload
	if err != nil {
		return fmt.Errorf("S3 upload failed: %v", err)
	}

	// Success!
	job.ProcessedRows = rowCount
	job.ResultKey = key

	log.Printf("[Worker] ✓ Export completed successfully:")
	log.Printf("    - Resource: %s", job.Resource)
	log.Printf("    - Rows exported: %d", rowCount)
	log.Printf("    - Format: %s", config.Format)
	log.Printf("    - S3 key: %s", key)
	log.Printf("    - Upload duration: %v", uploadDuration)

	return nil
}
