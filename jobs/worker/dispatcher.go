package worker

import (
	"log"
	"time"

	"github.com/gothinkster/golang-gin-realworld-example-app/common"
	"github.com/gothinkster/golang-gin-realworld-example-app/jobs"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StartWorker initializes the background job processor
func StartWorker() {
	go func() {
		log.Println("[Worker] Started background job processor")
		for {
			processNextJob()
			// Poll every 1 second to avoid hammering the DB
			time.Sleep(1 * time.Second)
		}
	}()
}

func processNextJob() {
	db := common.GetDB()
	var job jobs.Job

	// TRANSACTION: Find a PENDING job and lock it
	err := db.Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ?", jobs.StatusPending).
			Order("created_at ASC").
			First(&job)

		if result.Error != nil {
			// No jobs found, or DB error
			return result.Error
		}

		// Immediately mark as PROCESSING inside the transaction
		job.Status = jobs.StatusProcessing
		if err := tx.Save(&job).Error; err != nil {
			return err
		}

		return nil
	})

	// If no job found (RecordNotFound), just return and wait for next tick
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			log.Printf("[Worker] Error polling job: %v", err)
		}
		return
	}

	// Job found! Hand it off to the processor
	log.Printf("[Worker] Picked up Job %s (%s %s)", job.ID, job.Type, job.Resource)
	executeJob(&job)
}
