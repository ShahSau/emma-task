package routers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gothinkster/golang-gin-realworld-example-app/common"
	"github.com/gothinkster/golang-gin-realworld-example-app/jobs"
)

// GetJobStatus (GET /v1/exports/:job_id)
func GetJobStatus(c *gin.Context) {
	jobID := c.Param("job_id")
	db := common.GetDB()

	var job jobs.Job
	if err := db.Where("job_id = ?", jobID).First(&job).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	response := gin.H{
		"job_id":         job.ID,
		"type":           job.Type,
		"resource":       job.Resource,
		"status":         job.Status,
		"processed_rows": job.ProcessedRows,
		"failed_rows":    job.FailedRows,
		"created_at":     job.CreatedAt,
		"updated_at":     job.UpdatedAt,
	}

	if job.Status == jobs.StatusFailed {
		response["error_message"] = job.ErrorMessage
	}

	// If the job produced a file (Export Result OR Import Error Report), generate a URL
	if job.Status == jobs.StatusCompleted || job.FailedRows > 0 {
		if job.ResultKey != "" {
			url, err := common.GetPresignedURL(job.ResultKey)
			if err == nil {
				response["download_url"] = url
			}
		}
	}

	c.JSON(http.StatusOK, response)
}
