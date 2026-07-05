package models

import "time"

type Comment struct {
	CommentID       string     `json:"comment_id"`
	DocumentID      string     `json:"document_id"`
	ChunkID         *string    `json:"chunk_id,omitempty"`
	ParentCommentID *string    `json:"parent_comment_id,omitempty"`
	RangeStart      int        `json:"range_start"`
	RangeEnd        int        `json:"range_end"`
	Body            string     `json:"body"`
	Resolved        bool       `json:"resolved"`
	ResolvedBy      *string    `json:"resolved_by,omitempty"`
	ResolvedAt      *time.Time `json:"resolved_at,omitempty"`
	CreatedBy       string     `json:"created_by"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
