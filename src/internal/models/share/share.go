package models

import "time"

type ShareLink struct {
	ShareID    string     `json:"share_id"`
	Token      string     `json:"token"`
	DocumentID string     `json:"document_id"`
	Permission string     `json:"permission"`
	CreatedBy  string     `json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}
