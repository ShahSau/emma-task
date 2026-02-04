package jobs

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Job Status Constants
const (
	StatusPending    = "PENDING"
	StatusProcessing = "PROCESSING"
	StatusCompleted  = "COMPLETED"
	StatusFailed     = "FAILED"
)

// Job Type Constants
const (
	TypeImport = "IMPORT"
	TypeExport = "EXPORT"
)

type Job struct {
	ID       uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	Type     string    `gorm:"size:20;index;not null" json:"type"`
	Resource string    `gorm:"size:50;not null" json:"resource"`
	Status   string    `gorm:"size:20;index;not null" json:"status"`

	// S3 Keys
	SourceKey string `json:"-"` // File uploaded by user
	ResultKey string `json:"-"` // Exported file OR Error Report

	// Counters
	TotalRows     int `gorm:"default:0" json:"total_rows"`
	ProcessedRows int `gorm:"default:0" json:"processed_rows"`
	FailedRows    int `gorm:"default:0" json:"failed_rows"`

	// Reliability
	IdempotencyKey string `gorm:"size:255;uniqueIndex" json:"-"`
	ErrorMessage   string `gorm:"type:text" json:"error_message,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate is a GORM hook to generate UUIDs
func (j *Job) BeforeCreate(tx *gorm.DB) (err error) {
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	return
}
