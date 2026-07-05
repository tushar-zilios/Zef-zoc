package db

import (
	"context"
	"fmt"

	dbpkg "zoc/src/internal/db"
	models "zoc/src/internal/models/document"

	"github.com/jackc/pgx/v5"
)

const docCols = `document_id, folder_id, name, mime_type, size_bytes, storage_key, version, checksum, created_by, updated_by, created_at, updated_at, deleted_at, archived_at, is_template`

func scanDoc(row pgx.Row) (*models.Document, error) {
	d := &models.Document{}
	if err := row.Scan(&d.DocumentID, &d.FolderID, &d.Name, &d.MimeType, &d.SizeBytes, &d.StorageKey, &d.Version, &d.Checksum, &d.CreatedBy, &d.UpdatedBy, &d.CreatedAt, &d.UpdatedAt, &d.DeletedAt, &d.ArchivedAt, &d.IsTemplate); err != nil {
		return nil, err
	}
	return d, nil
}

func CreateDocument(ctx context.Context, folderID *string, name, mimeType string, sizeBytes int64, storageKey, checksum, createdBy string) (*models.Document, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}

	row := pool.QueryRow(ctx, `
		INSERT INTO public.documents (folder_id, name, mime_type, size_bytes, storage_key, checksum, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		RETURNING `+docCols,
		folderID, name, mimeType, sizeBytes, storageKey, checksum, createdBy)
	d, err := scanDoc(row)
	if err != nil {
		return nil, err
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO public.zoc_document_versions (document_id, version, storage_key, size_bytes, checksum, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, d.DocumentID, d.Version, d.StorageKey, d.SizeBytes, d.Checksum, createdBy)
	if err != nil {
		return nil, err
	}

	return d, nil
}

func ListDocuments(ctx context.Context, folderID *string) ([]models.Document, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}

	var rows pgx.Rows
	var err error
	if folderID != nil {
		rows, err = pool.Query(ctx, `
			SELECT `+docCols+`
			FROM public.documents WHERE folder_id = $1 AND deleted_at IS NULL ORDER BY name`, *folderID)
	} else {
		rows, err = pool.Query(ctx, `
			SELECT `+docCols+`
			FROM public.documents WHERE folder_id IS NULL AND deleted_at IS NULL ORDER BY name`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Document
	for rows.Next() {
		d, err := scanDoc(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *d)
	}
	return list, rows.Err()
}

func GetDocument(ctx context.Context, documentID string) (*models.Document, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}
	row := pool.QueryRow(ctx, `SELECT `+docCols+` FROM public.documents WHERE document_id = $1`, documentID)
	return scanDoc(row)
}

// UpdateDocument renames/moves a document and, if storageKey is non-empty, bumps
// its version and records the change in zoc_document_versions.
func UpdateDocument(ctx context.Context, documentID string, name *string, folderID *string, storageKey, checksum *string, sizeBytes *int64, updatedBy string) (*models.Document, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}

	existing, err := GetDocument(ctx, documentID)
	if err != nil {
		return nil, err
	}

	newName := existing.Name
	if name != nil {
		newName = *name
	}
	newFolderID := existing.FolderID
	if folderID != nil {
		newFolderID = folderID
	}
	newVersion := existing.Version
	newStorageKey := existing.StorageKey
	newChecksum := existing.Checksum
	newSize := existing.SizeBytes
	bumpVersion := storageKey != nil && *storageKey != "" && *storageKey != existing.StorageKey
	if bumpVersion {
		newVersion++
		newStorageKey = *storageKey
		if checksum != nil {
			newChecksum = *checksum
		}
		if sizeBytes != nil {
			newSize = *sizeBytes
		}
	}

	row := pool.QueryRow(ctx, `
		UPDATE public.documents
		SET name = $1, folder_id = $2, version = $3, storage_key = $4, checksum = $5, size_bytes = $6, updated_by = $7, updated_at = NOW()
		WHERE document_id = $8
		RETURNING `+docCols,
		newName, newFolderID, newVersion, newStorageKey, newChecksum, newSize, updatedBy, documentID)
	d, err := scanDoc(row)
	if err != nil {
		return nil, err
	}

	if bumpVersion {
		if _, err := pool.Exec(ctx, `
			INSERT INTO public.zoc_document_versions (document_id, version, storage_key, size_bytes, checksum, created_by)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, d.DocumentID, d.Version, d.StorageKey, d.SizeBytes, d.Checksum, updatedBy); err != nil {
			return nil, err
		}
	}

	return d, nil
}

func SoftDeleteDocument(ctx context.Context, documentID string) error {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return fmt.Errorf("database not configured")
	}
	_, err := pool.Exec(ctx, `UPDATE public.documents SET deleted_at = NOW() WHERE document_id = $1`, documentID)
	return err
}

func RestoreDocument(ctx context.Context, documentID string) error {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return fmt.Errorf("database not configured")
	}
	_, err := pool.Exec(ctx, `UPDATE public.documents SET deleted_at = NULL WHERE document_id = $1`, documentID)
	return err
}

func ListTrashedDocuments(ctx context.Context, userID string) ([]models.Document, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}
	rows, err := pool.Query(ctx, `SELECT `+docCols+` FROM public.documents WHERE deleted_at IS NOT NULL AND created_by = $1 ORDER BY deleted_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Document
	for rows.Next() {
		d, err := scanDoc(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *d)
	}
	return list, rows.Err()
}

func SetArchived(ctx context.Context, documentID string, archived bool) (*models.Document, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}
	var row pgx.Row
	if archived {
		row = pool.QueryRow(ctx, `UPDATE public.documents SET archived_at = NOW() WHERE document_id = $1 RETURNING `+docCols, documentID)
	} else {
		row = pool.QueryRow(ctx, `UPDATE public.documents SET archived_at = NULL WHERE document_id = $1 RETURNING `+docCols, documentID)
	}
	return scanDoc(row)
}

// DuplicateDocument copies a document's metadata and chunk content into a new
// document owned by the requester, resetting version to 1.
func DuplicateDocument(ctx context.Context, documentID, requestedBy string) (*models.Document, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}

	src, err := GetDocument(ctx, documentID)
	if err != nil {
		return nil, err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		INSERT INTO public.documents (folder_id, name, mime_type, size_bytes, storage_key, checksum, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		RETURNING `+docCols,
		src.FolderID, src.Name+" (copy)", src.MimeType, src.SizeBytes, src.StorageKey, src.Checksum, requestedBy)
	dst, err := scanDoc(row)
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO public.zoc_document_chunks (document_id, chunk_index, chunk_type, content)
		SELECT $1, chunk_index, chunk_type, content FROM public.zoc_document_chunks WHERE document_id = $2
	`, dst.DocumentID, src.DocumentID); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO public.zoc_document_versions (document_id, version, storage_key, size_bytes, checksum, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, dst.DocumentID, dst.Version, dst.StorageKey, dst.SizeBytes, dst.Checksum, requestedBy); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return dst, nil
}

func BulkMoveDocuments(ctx context.Context, documentIDs []string, folderID *string) error {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return fmt.Errorf("database not configured")
	}
	_, err := pool.Exec(ctx, `UPDATE public.documents SET folder_id = $1, updated_at = NOW() WHERE document_id = ANY($2)`, folderID, documentIDs)
	return err
}

func ListDocumentVersions(ctx context.Context, documentID string) ([]models.DocumentVersion, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}

	rows, err := pool.Query(ctx, `
		SELECT version_id, document_id, version, storage_key, size_bytes, checksum, created_by, created_at, chunks_snapshot
		FROM public.zoc_document_versions WHERE document_id = $1 ORDER BY version DESC`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.DocumentVersion
	for rows.Next() {
		var v models.DocumentVersion
		if err := rows.Scan(&v.VersionID, &v.DocumentID, &v.Version, &v.StorageKey, &v.SizeBytes, &v.Checksum, &v.CreatedBy, &v.CreatedAt, &v.ChunksSnapshot); err != nil {
			return nil, err
		}
		list = append(list, v)
	}
	return list, rows.Err()
}

func GetDocumentVersion(ctx context.Context, documentID string, version int) (*models.DocumentVersion, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}
	var v models.DocumentVersion
	row := pool.QueryRow(ctx, `
		SELECT version_id, document_id, version, storage_key, size_bytes, checksum, created_by, created_at, chunks_snapshot
		FROM public.zoc_document_versions WHERE document_id = $1 AND version = $2`, documentID, version)
	if err := row.Scan(&v.VersionID, &v.DocumentID, &v.Version, &v.StorageKey, &v.SizeBytes, &v.Checksum, &v.CreatedBy, &v.CreatedAt, &v.ChunksSnapshot); err != nil {
		return nil, err
	}
	return &v, nil
}

// SnapshotChunksVersion bumps the document version and records the given
// chunk content as a JSON snapshot, for chunk-based (rich-text) docs where
// UpdateDocument's storage-key bump logic doesn't apply.
func SnapshotChunksVersion(ctx context.Context, documentID string, chunksJSON []byte, createdBy string) (int, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return 0, fmt.Errorf("database not configured")
	}

	var newVersion int
	row := pool.QueryRow(ctx, `UPDATE public.documents SET version = version + 1 WHERE document_id = $1 RETURNING version`, documentID)
	if err := row.Scan(&newVersion); err != nil {
		return 0, err
	}

	_, err := pool.Exec(ctx, `
		INSERT INTO public.zoc_document_versions (document_id, version, storage_key, size_bytes, checksum, created_by, chunks_snapshot)
		VALUES ($1, $2, '', 0, '', $3, $4::jsonb)
	`, documentID, newVersion, createdBy, string(chunksJSON))
	if err != nil {
		return 0, err
	}
	return newVersion, nil
}

// UpdateSearchText refreshes the denormalized search_text column used by the
// generated search_tsv column, rolled up from chunk plain text.
func UpdateSearchText(ctx context.Context, documentID, text string) error {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return fmt.Errorf("database not configured")
	}
	_, err := pool.Exec(ctx, `UPDATE public.documents SET search_text = $1 WHERE document_id = $2`, text, documentID)
	return err
}

func Search(ctx context.Context, query, folderID, tagID string) ([]models.Document, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}

	sql := `SELECT ` + docCols + ` FROM public.documents d WHERE deleted_at IS NULL AND search_tsv @@ plainto_tsquery('english', $1)`
	args := []any{query}
	if folderID != "" {
		args = append(args, folderID)
		sql += fmt.Sprintf(" AND folder_id = $%d", len(args))
	}
	if tagID != "" {
		args = append(args, tagID)
		sql += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM public.document_tags dt WHERE dt.document_id = d.document_id AND dt.tag_id = $%d)", len(args))
	}
	sql += " ORDER BY ts_rank(search_tsv, plainto_tsquery('english', $1)) DESC LIMIT 50"

	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Document
	for rows.Next() {
		d, err := scanDoc(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *d)
	}
	return list, rows.Err()
}

func SetIsTemplate(ctx context.Context, documentID string, isTemplate bool) error {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return fmt.Errorf("database not configured")
	}
	_, err := pool.Exec(ctx, `UPDATE public.documents SET is_template = $1 WHERE document_id = $2`, isTemplate, documentID)
	return err
}

func ListTemplates(ctx context.Context, userID string) ([]models.Document, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}
	rows, err := pool.Query(ctx, `SELECT `+docCols+` FROM public.documents WHERE is_template = true AND deleted_at IS NULL AND created_by = $1 ORDER BY name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Document
	for rows.Next() {
		d, err := scanDoc(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *d)
	}
	return list, rows.Err()
}
