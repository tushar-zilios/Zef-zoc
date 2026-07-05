package db

import (
	"context"
	"fmt"

	dbpkg "zoc/src/internal/db"
	models "zoc/src/internal/models/comment"

	"github.com/jackc/pgx/v5"
)

const selectCols = `comment_id, document_id, chunk_id, parent_comment_id, range_start, range_end, body, resolved, resolved_by, resolved_at, created_by, created_at, updated_at`

func scanComment(row pgx.Row) (*models.Comment, error) {
	c := &models.Comment{}
	if err := row.Scan(&c.CommentID, &c.DocumentID, &c.ChunkID, &c.ParentCommentID, &c.RangeStart, &c.RangeEnd, &c.Body, &c.Resolved, &c.ResolvedBy, &c.ResolvedAt, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return c, nil
}

func CreateComment(ctx context.Context, documentID string, chunkID *string, parentCommentID *string, rangeStart, rangeEnd int, body, createdBy string) (*models.Comment, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}
	row := pool.QueryRow(ctx, `
		INSERT INTO public.zoc_comments (document_id, chunk_id, parent_comment_id, range_start, range_end, body, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+selectCols,
		documentID, chunkID, parentCommentID, rangeStart, rangeEnd, body, createdBy)
	return scanComment(row)
}

func ListComments(ctx context.Context, documentID string) ([]models.Comment, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}
	rows, err := pool.Query(ctx, `SELECT `+selectCols+` FROM public.zoc_comments WHERE document_id = $1 ORDER BY created_at`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Comment
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *c)
	}
	return list, rows.Err()
}

func ResolveComment(ctx context.Context, commentID string, resolved bool, resolvedBy string) (*models.Comment, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}
	var row pgx.Row
	if resolved {
		row = pool.QueryRow(ctx, `
			UPDATE public.zoc_comments SET resolved = true, resolved_by = $1, resolved_at = NOW(), updated_at = NOW()
			WHERE comment_id = $2 RETURNING `+selectCols, resolvedBy, commentID)
	} else {
		row = pool.QueryRow(ctx, `
			UPDATE public.zoc_comments SET resolved = false, resolved_by = NULL, resolved_at = NULL, updated_at = NOW()
			WHERE comment_id = $1 RETURNING `+selectCols, commentID)
	}
	return scanComment(row)
}

func DeleteComment(ctx context.Context, commentID string) error {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return fmt.Errorf("database not configured")
	}
	_, err := pool.Exec(ctx, `DELETE FROM public.zoc_comments WHERE comment_id = $1`, commentID)
	return err
}
