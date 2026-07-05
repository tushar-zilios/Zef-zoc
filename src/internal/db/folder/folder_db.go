package db

import (
	"context"
	"fmt"

	dbpkg "zoc/src/internal/db"
	models "zoc/src/internal/models/folder"

	"github.com/jackc/pgx/v5"
)

func CreateFolder(ctx context.Context, parentID *string, name, createdBy string) (*models.Folder, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}

	var parentPath string
	if parentID != nil {
		if err := pool.QueryRow(ctx, `SELECT path FROM public.folders WHERE folder_id = $1`, *parentID).Scan(&parentPath); err != nil {
			return nil, fmt.Errorf("failed to resolve parent folder: %w", err)
		}
	}

	f := &models.Folder{}
	row := pool.QueryRow(ctx, `
		INSERT INTO public.folders (parent_id, name, path, created_by)
		VALUES ($1, $2, '', $3)
		RETURNING folder_id, parent_id, name, path, created_by, created_at, updated_at
	`, parentID, name, createdBy)
	if err := row.Scan(&f.FolderID, &f.ParentID, &f.Name, &f.Path, &f.CreatedBy, &f.CreatedAt, &f.UpdatedAt); err != nil {
		return nil, err
	}

	newPath := parentPath + f.FolderID + "/"
	if _, err := pool.Exec(ctx, `UPDATE public.folders SET path = $1 WHERE folder_id = $2`, newPath, f.FolderID); err != nil {
		return nil, err
	}
	f.Path = newPath

	return f, nil
}

const folderCols = `folder_id, parent_id, name, path, created_by, created_at, updated_at, deleted_at`

func scanFolder(row pgx.Row) (*models.Folder, error) {
	f := &models.Folder{}
	if err := row.Scan(&f.FolderID, &f.ParentID, &f.Name, &f.Path, &f.CreatedBy, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt); err != nil {
		return nil, err
	}
	return f, nil
}

func ListFolders(ctx context.Context, parentID *string) ([]models.Folder, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}

	var rows pgx.Rows
	var err error
	if parentID != nil {
		rows, err = pool.Query(ctx, `
			SELECT `+folderCols+`
			FROM public.folders WHERE parent_id = $1 AND deleted_at IS NULL ORDER BY name`, *parentID)
	} else {
		rows, err = pool.Query(ctx, `
			SELECT `+folderCols+`
			FROM public.folders WHERE parent_id IS NULL AND deleted_at IS NULL ORDER BY name`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Folder
	for rows.Next() {
		f, err := scanFolder(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *f)
	}
	return list, rows.Err()
}

func GetFolder(ctx context.Context, folderID string) (*models.Folder, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}
	row := pool.QueryRow(ctx, `SELECT `+folderCols+` FROM public.folders WHERE folder_id = $1`, folderID)
	return scanFolder(row)
}

func RenameFolder(ctx context.Context, folderID, name string) error {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return fmt.Errorf("database not configured")
	}
	_, err := pool.Exec(ctx, `UPDATE public.folders SET name = $1, updated_at = NOW() WHERE folder_id = $2`, name, folderID)
	return err
}

func MoveFolder(ctx context.Context, folderID string, parentID *string) error {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return fmt.Errorf("database not configured")
	}
	_, err := pool.Exec(ctx, `UPDATE public.folders SET parent_id = $1, updated_at = NOW() WHERE folder_id = $2`, parentID, folderID)
	return err
}

// SoftDeleteFolder marks a folder (and its direct document children) as
// deleted. Nested subfolders/documents follow on their own restore/delete calls.
func SoftDeleteFolder(ctx context.Context, folderID string) error {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return fmt.Errorf("database not configured")
	}
	_, err := pool.Exec(ctx, `UPDATE public.folders SET deleted_at = NOW() WHERE folder_id = $1`, folderID)
	return err
}

func RestoreFolder(ctx context.Context, folderID string) error {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return fmt.Errorf("database not configured")
	}
	_, err := pool.Exec(ctx, `UPDATE public.folders SET deleted_at = NULL WHERE folder_id = $1`, folderID)
	return err
}

func ListTrashedFolders(ctx context.Context, userID string) ([]models.Folder, error) {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return nil, fmt.Errorf("database not configured")
	}
	rows, err := pool.Query(ctx, `SELECT `+folderCols+` FROM public.folders WHERE deleted_at IS NOT NULL AND created_by = $1 ORDER BY deleted_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Folder
	for rows.Next() {
		f, err := scanFolder(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *f)
	}
	return list, rows.Err()
}

// DeleteFolder permanently removes a folder (hard delete), for use once
// something is already in trash and the user empties it.
func DeleteFolder(ctx context.Context, folderID string) error {
	pool := dbpkg.GetZocPoolOrNil()
	if pool == nil {
		return fmt.Errorf("database not configured")
	}
	_, err := pool.Exec(ctx, `DELETE FROM public.folders WHERE folder_id = $1`, folderID)
	return err
}
