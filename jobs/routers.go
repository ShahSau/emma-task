package jobs

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/gothinkster/golang-gin-realworld-example-app/common"
	"gorm.io/gorm"
)

func JobsRegister(router *gin.RouterGroup) {
	router.POST("/imports", CreateImportJob)
	router.GET("/imports/:id", GetJobStatus)
	router.GET("/imports/:id/errors", GetJobErrors)
}

// CreateImportJob handles POST /v1/imports
func CreateImportJob(c *gin.Context) {
	db := common.GetDB()

	// Idempotency Check
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey != "" {
		var existingJob Job
		if err := db.Where("idempotency_key = ?", idempotencyKey).First(&existingJob).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Job with this Idempotency-Key already exists", "job_id": existingJob.ID})
			return
		}
	}

	// Parse Multipart Form (Max 10MB header, but stream body)
	// We assume "resource" is a form field and "file" is the file
	resource := c.PostForm("resource")
	if resource == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resource field is required (users, articles, comments)"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file upload is required"})
		return
	}

	// Open File Stream
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open file"})
		return
	}
	defer file.Close()

	// Upload Stream to S3 (LocalStack)
	s3Client := common.GetS3()
	uploader := manager.NewUploader(s3Client)
	bucket := os.Getenv("S3_BUCKET")
	key := fmt.Sprintf("imports/%s/%d_%s", resource, time.Now().Unix(), filepath.Base(fileHeader.Filename))

	_, err = uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upload to storage", "details": err.Error()})
		return
	}

	// Create Job Record
	job := Job{
		Type:           TypeImport,
		Resource:       resource,
		Status:         StatusPending,
		SourceKey:      key,
		IdempotencyKey: idempotencyKey,
	}

	if err := db.Create(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create job record"})
		return
	}

	// Return 202 Accepted
	c.JSON(http.StatusAccepted, gin.H{
		"message": "Import job accepted",
		"job_id":  job.ID,
		"status":  job.Status,
	})
}

// GetJobStatus handles GET /v1/imports/:id
func GetJobStatus(c *gin.Context) {
	id := c.Param("id")
	db := common.GetDB()
	var job Job

	if err := db.First(&job, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	c.JSON(http.StatusOK, job)
}

// GetJobErrors handles GET /v1/imports/:id/errors
// Redirects to the S3 Presigned URL of the error report
func GetJobErrors(c *gin.Context) {
	id := c.Param("id")
	db := common.GetDB()
	var job Job

	if err := db.First(&job, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	if job.ResultKey == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "No error report available for this job"})
		return
	}

	// Generate Presigned URL
	s3Client := common.GetS3()
	presignClient := s3.NewPresignClient(s3Client)
	bucket := os.Getenv("S3_BUCKET")

	req, err := presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(job.ResultKey),
	}, s3.WithPresignExpires(15*time.Minute))

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate download link"})
		return
	}

	// Redirect user to the S3 URL
	c.Redirect(http.StatusFound, req.URL)
}
