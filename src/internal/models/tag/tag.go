package models

import "time"

type Tag struct {
	TagID     string    `json:"tag_id"`
	Name      string    `json:"name"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}
