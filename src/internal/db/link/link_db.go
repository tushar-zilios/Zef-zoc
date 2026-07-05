package db

import (
	"context"
	"fmt"

	dbpkg "zoc/src/internal/db"
)

// ReplaceOutgoingLinks replaces all backlink rows sourced from documentID with
// the given set of target document IDs (typically parsed from @doc-link mentions).
func ReplaceOutgoingLinks(ctx context.Context, documentID string, targetIDs []string) error {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return fmt.Errorf("database not configured")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM public.document_links WHERE source_document_id = $1`, documentID); err != nil {
		return err
	}
	for _, targetID := range targetIDs {
		if targetID == documentID {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO public.document_links (source_document_id, target_document_id)
			VALUES ($1, $2) ON CONFLICT DO NOTHING`, documentID, targetID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func ListBacklinks(ctx context.Context, documentID string) ([]string, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}
	rows, err := pool.Query(ctx, `SELECT DISTINCT source_document_id FROM public.document_links WHERE target_document_id = $1`, documentID)
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
