package models

import "time"

const (
	DataDumpStatusPending    = "Pending…"
	DataDumpStatusProcessing = "Processing coding activity…"
	DataDumpStatusUploading  = "Uploading…"
	DataDumpStatusCompleted  = "Completed"

	DataDumpTypeHeartbeats = "heartbeats"
	DataDumpTypeDaily      = "daily"
)

type DataDump struct {
	ID              string     `json:"id" gorm:"primary_key"`
	UserID          string     `json:"user_id" gorm:"not null; index:idx_data_dump_user"`
	Status          string     `json:"status" gorm:"default:'Pending…'"`
	PercentComplete float32    `json:"percent_complete" gorm:"default:0"`
	DownloadUrl     string     `json:"download_url"`
	Type            string     `json:"type" gorm:"default:'heartbeats'"`
	IsProcessing    bool       `json:"is_processing" gorm:"default:false; type:bool"`
	IsStuck         bool       `json:"is_stuck" gorm:"default:false; type:bool"`
	HasFailed       bool       `json:"has_failed" gorm:"default:false; type:bool"`
	Expires         *time.Time `json:"expires"`
	CreatedAt       time.Time  `json:"created_at" gorm:"default:CURRENT_TIMESTAMP"`
}
