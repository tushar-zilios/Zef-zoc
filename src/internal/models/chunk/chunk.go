package models

import (
	"encoding/json"
	"time"
)

type DocumentChunk struct {
	ChunkID    string          `json:"chunk_id"`
	DocumentID string          `json:"document_id"`
	ChunkIndex int             `json:"chunk_index"`
	ChunkType  string          `json:"chunk_type"`
	Content    json.RawMessage `json:"content"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

type ChunkInput struct {
	ChunkIndex int             `json:"chunk_index"`
	ChunkType  string          `json:"chunk_type"`
	Content    json.RawMessage `json:"content"`
}
