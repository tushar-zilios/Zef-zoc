package models

import (
	"encoding/json"
	"time"
)

type Document struct {
	DocumentID string     `json:"document_id"`
	FolderID   *string    `json:"folder_id,omitempty"`
	Name       string     `json:"name"`
	MimeType   string     `json:"mime_type"`
	SizeBytes  int64      `json:"size_bytes"`
	StorageKey string     `json:"storage_key"`
	Version    int        `json:"version"`
	Checksum   string     `json:"checksum,omitempty"`
	CreatedBy  string     `json:"created_by"`
	UpdatedBy  string     `json:"updated_by"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
	IsTemplate bool       `json:"is_template"`
}

type DocumentVersion struct {
	VersionID      string          `json:"version_id"`
	DocumentID     string          `json:"document_id"`
	Version        int             `json:"version"`
	StorageKey     string          `json:"storage_key"`
	SizeBytes      int64           `json:"size_bytes"`
	Checksum       string          `json:"checksum,omitempty"`
	CreatedBy      string          `json:"created_by"`
	CreatedAt      time.Time       `json:"created_at"`
	ChunksSnapshot json.RawMessage `json:"chunks_snapshot,omitempty"`
}
