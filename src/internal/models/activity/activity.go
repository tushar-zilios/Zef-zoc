package models

import (
	"encoding/json"
	"time"
)

type Activity struct {
	ActivityID string          `json:"activity_id"`
	DocumentID string          `json:"document_id"`
	UserID     string          `json:"user_id"`
	Action     string          `json:"action"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}
