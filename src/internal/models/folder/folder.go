package models

import "time"

type Folder struct {
	FolderID  string     `json:"folder_id"`
	ParentID  *string    `json:"parent_id,omitempty"`
	Name      string     `json:"name"`
	Path      string     `json:"path"`
	CreatedBy string     `json:"created_by"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}
