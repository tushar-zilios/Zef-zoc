package db

import (
	"context"
	"encoding/json"
	"fmt"

	dbpkg "zoc/src/internal/db"
	models "zoc/src/internal/models/activity"
)

func LogActivity(ctx context.Context, documentID, userID, action string, metadata any) error {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return fmt.Errorf("database not configured")
	}
	var metaJSON []byte
	if metadata != nil {
		var err error
		metaJSON, err = json.Marshal(metadata)
		if err != nil {
			return err
		}
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO public.document_activity (document_id, user_id, action, metadata)
		VALUES ($1, $2, $3, $4)`, documentID, userID, action, metaJSON)
	return err
}

func ListActivity(ctx context.Context, documentID string) ([]models.Activity, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}
	rows, err := pool.Query(ctx, `
		SELECT activity_id, document_id, user_id, action, metadata, created_at
		FROM public.document_activity WHERE document_id = $1 ORDER BY created_at DESC LIMIT 200`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Activity
	for rows.Next() {
		var a models.Activity
		if err := rows.Scan(&a.ActivityID, &a.DocumentID, &a.UserID, &a.Action, &a.Metadata, &a.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, rows.Err()
}
