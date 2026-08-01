package db

import (
	"context"
	"fmt"
	"time"

	dbpkg "zoc/src/internal/db"
	models "zoc/src/internal/models/share"

	"github.com/jackc/pgx/v5"
)

const shareCols = `share_id, token, document_id, permission, created_by, created_at, expires_at, revoked_at`

func scanShare(row pgx.Row) (*models.ShareLink, error) {
	s := &models.ShareLink{}
	if err := row.Scan(&s.ShareID, &s.Token, &s.DocumentID, &s.Permission, &s.CreatedBy, &s.CreatedAt, &s.ExpiresAt, &s.RevokedAt); err != nil {
		return nil, err
	}
	return s, nil
}

func CreateShareLink(ctx context.Context, documentID, token, permission, createdBy string, expiresAt *time.Time) (*models.ShareLink, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}
	row := pool.QueryRow(ctx, `
		INSERT INTO public.share_links (document_id, token, permission, created_by, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+shareCols, documentID, token, permission, createdBy, expiresAt)
	return scanShare(row)
}

func ListShareLinks(ctx context.Context, documentID string) ([]models.ShareLink, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}
	rows, err := pool.Query(ctx, `SELECT `+shareCols+` FROM public.share_links WHERE document_id = $1 ORDER BY created_at DESC`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.ShareLink
	for rows.Next() {
		s, err := scanShare(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *s)
	}
	return list, rows.Err()
}

func RevokeShareLink(ctx context.Context, token string) error {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return fmt.Errorf("database not configured")
	}
	_, err := pool.Exec(ctx, `UPDATE public.share_links SET revoked_at = NOW() WHERE token = $1`, token)
	return err
}

const docShareCols = `document_id, user_id, permission, created_by, created_at`

func scanDocShare(row pgx.Row) (*models.DocumentShare, error) {
	s := &models.DocumentShare{}
	if err := row.Scan(&s.DocumentID, &s.UserID, &s.Permission, &s.CreatedBy, &s.CreatedAt); err != nil {
		return nil, err
	}
	return s, nil
}

// AddDocumentShare grants (or updates) a specific user's standing access to a document.
func AddDocumentShare(ctx context.Context, documentID, userID, permission, createdBy string) (*models.DocumentShare, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}
	row := pool.QueryRow(ctx, `
		INSERT INTO public.document_shares (document_id, user_id, permission, created_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (document_id, user_id) DO UPDATE SET permission = EXCLUDED.permission
		RETURNING `+docShareCols, documentID, userID, permission, createdBy)
	return scanDocShare(row)
}

func ListDocumentShares(ctx context.Context, documentID string) ([]models.DocumentShare, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}
	rows, err := pool.Query(ctx, `SELECT `+docShareCols+` FROM public.document_shares WHERE document_id = $1 ORDER BY created_at DESC`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.DocumentShare
	for rows.Next() {
		s, err := scanDocShare(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *s)
	}
	return list, rows.Err()
}

func RemoveDocumentShare(ctx context.Context, documentID, userID string) error {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return fmt.Errorf("database not configured")
	}
	_, err := pool.Exec(ctx, `DELETE FROM public.document_shares WHERE document_id = $1 AND user_id = $2`, documentID, userID)
	return err
}

// GetUserPermission returns the permission level a user was granted on a document,
// or pgx.ErrNoRows if the user has no standing share on it.
func GetUserPermission(ctx context.Context, documentID, userID string) (string, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return "", fmt.Errorf("database not configured")
	}
	var permission string
	err := pool.QueryRow(ctx, `SELECT permission FROM public.document_shares WHERE document_id = $1 AND user_id = $2`, documentID, userID).Scan(&permission)
	return permission, err
}

// GetValidShareLink returns the share link only if it isn't revoked or expired.
func GetValidShareLink(ctx context.Context, token string) (*models.ShareLink, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}
	row := pool.QueryRow(ctx, `
		SELECT `+shareCols+` FROM public.share_links
		WHERE token = $1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > NOW())`, token)
	return scanShare(row)
}
