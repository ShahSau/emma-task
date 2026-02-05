package worker

import (
	"fmt"
	"log"
	"time"

	"github.com/gothinkster/golang-gin-realworld-example-app/common"
	"github.com/gothinkster/golang-gin-realworld-example-app/jobs"
	"github.com/gothinkster/golang-gin-realworld-example-app/jobs/core"
)

func executeJob(job *jobs.Job) {
	db := common.GetDB()
	var err error

	// Route based on Job Type
	switch job.Type {
	case jobs.TypeImport:
		err = core.ProcessImport(job)
	case jobs.TypeExport:
		err = processExportStub(job)
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
		job.ProcessedRows = job.TotalRows
	}

	job.UpdatedAt = time.Now()
	db.Save(job)
}

func processImportStub(job *jobs.Job) error {
	log.Printf("   -> Simulating IMPORT for %s...", job.Resource)
	time.Sleep(2 * time.Second) // Fake work

	// Simulate success
	job.TotalRows = 100
	return nil
}

func processExportStub(job *jobs.Job) error {
	log.Printf("   -> Simulating EXPORT for %s...", job.Resource)
	time.Sleep(2 * time.Second) // Fake work
	return nil
}
