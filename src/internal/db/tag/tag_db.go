package db

import (
	"context"
	"fmt"

	dbpkg "zoc/src/internal/db"
	models "zoc/src/internal/models/tag"
)

func CreateTag(ctx context.Context, name, createdBy string) (*models.Tag, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}
	t := &models.Tag{}
	row := pool.QueryRow(ctx, `
		INSERT INTO public.tags (name, created_by) VALUES ($1, $2)
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		RETURNING tag_id, name, created_by, created_at`, name, createdBy)
	if err := row.Scan(&t.TagID, &t.Name, &t.CreatedBy, &t.CreatedAt); err != nil {
		return nil, err
	}
	return t, nil
}

func ListTags(ctx context.Context, userID string) ([]models.Tag, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}
	rows, err := pool.Query(ctx, `SELECT tag_id, name, created_by, created_at FROM public.tags WHERE created_by = $1 ORDER BY name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Tag
	for rows.Next() {
		var t models.Tag
		if err := rows.Scan(&t.TagID, &t.Name, &t.CreatedBy, &t.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

func DeleteTag(ctx context.Context, tagID string) error {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return fmt.Errorf("database not configured")
	}
	_, err := pool.Exec(ctx, `DELETE FROM public.tags WHERE tag_id = $1`, tagID)
	return err
}

func AddDocumentTag(ctx context.Context, documentID, tagID string) error {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return fmt.Errorf("database not configured")
	}
	_, err := pool.Exec(ctx, `INSERT INTO public.document_tags (document_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, documentID, tagID)
	return err
}

func RemoveDocumentTag(ctx context.Context, documentID, tagID string) error {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return fmt.Errorf("database not configured")
	}
	_, err := pool.Exec(ctx, `DELETE FROM public.document_tags WHERE document_id = $1 AND tag_id = $2`, documentID, tagID)
	return err
}

func ListDocumentTags(ctx context.Context, documentID string) ([]models.Tag, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}
	rows, err := pool.Query(ctx, `
		SELECT t.tag_id, t.name, t.created_by, t.created_at
		FROM public.tags t JOIN public.document_tags dt ON dt.tag_id = t.tag_id
		WHERE dt.document_id = $1 ORDER BY t.name`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Tag
	for rows.Next() {
		var t models.Tag
		if err := rows.Scan(&t.TagID, &t.Name, &t.CreatedBy, &t.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

func RecordView(ctx context.Context, documentID, userID string) error {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return fmt.Errorf("database not configured")
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO public.document_views (document_id, user_id, viewed_at) VALUES ($1, $2, NOW())
		ON CONFLICT (document_id, user_id) DO UPDATE SET viewed_at = NOW()`, documentID, userID)
	return err
}

func ListRecentDocumentIDs(ctx context.Context, userID string, limit int) ([]string, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}
	rows, err := pool.Query(ctx, `SELECT document_id FROM public.document_views WHERE user_id = $1 ORDER BY viewed_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func StarDocument(ctx context.Context, documentID, userID string) error {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return fmt.Errorf("database not configured")
	}
	_, err := pool.Exec(ctx, `INSERT INTO public.document_stars (document_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, documentID, userID)
	return err
}

func UnstarDocument(ctx context.Context, documentID, userID string) error {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return fmt.Errorf("database not configured")
	}
	_, err := pool.Exec(ctx, `DELETE FROM public.document_stars WHERE document_id = $1 AND user_id = $2`, documentID, userID)
	return err
}

func ListStarredDocumentIDs(ctx context.Context, userID string) ([]string, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}
	rows, err := pool.Query(ctx, `SELECT document_id FROM public.document_stars WHERE user_id = $1 ORDER BY starred_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
