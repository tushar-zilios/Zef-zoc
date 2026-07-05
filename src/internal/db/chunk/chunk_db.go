package db

import (
	"context"
	"fmt"

	dbpkg "zoc/src/internal/db"
	models "zoc/src/internal/models/chunk"
)

func ListChunks(ctx context.Context, documentID string) ([]models.DocumentChunk, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}

	rows, err := pool.Query(ctx, `
		SELECT chunk_id, document_id, chunk_index, chunk_type, content, created_at, updated_at
		FROM public.zoc_document_chunks WHERE document_id = $1 ORDER BY chunk_index`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.DocumentChunk
	for rows.Next() {
		var c models.DocumentChunk
		if err := rows.Scan(&c.ChunkID, &c.DocumentID, &c.ChunkIndex, &c.ChunkType, &c.Content, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// ReplaceChunks atomically replaces all chunks for a document with the given set.
func ReplaceChunks(ctx context.Context, documentID string, chunks []models.ChunkInput) ([]models.DocumentChunk, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM public.zoc_document_chunks WHERE document_id = $1`, documentID); err != nil {
		return nil, err
	}

	var result []models.DocumentChunk
	for _, in := range chunks {
		chunkType := in.ChunkType
		if chunkType == "" {
			chunkType = "text"
		}
		var c models.DocumentChunk
		row := tx.QueryRow(ctx, `
			INSERT INTO public.zoc_document_chunks (document_id, chunk_index, chunk_type, content)
			VALUES ($1, $2, $3, $4)
			RETURNING chunk_id, document_id, chunk_index, chunk_type, content, created_at, updated_at
		`, documentID, in.ChunkIndex, chunkType, in.Content)
		if err := row.Scan(&c.ChunkID, &c.DocumentID, &c.ChunkIndex, &c.ChunkType, &c.Content, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, c)
	}

	if _, err := tx.Exec(ctx, `UPDATE public.documents SET updated_at = NOW() WHERE document_id = $1`, documentID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func UpsertYDocState(ctx context.Context, documentID string, state []byte) error {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return fmt.Errorf("database not configured")
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO public.zoc_document_ydoc_state (document_id, ydoc_state)
		VALUES ($1, $2)
		ON CONFLICT (document_id) DO UPDATE SET ydoc_state = $2, updated_at = NOW()
	`, documentID, state)
	return err
}

func GetYDocState(ctx context.Context, documentID string) ([]byte, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}
	var state []byte
	err := pool.QueryRow(ctx, `SELECT ydoc_state FROM public.zoc_document_ydoc_state WHERE document_id = $1`, documentID).Scan(&state)
	return state, err
}
