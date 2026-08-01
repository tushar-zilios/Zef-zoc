package db

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	dbpkg "zoc/src/internal/db"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func TestMain(m *testing.M) {
	_ = godotenv.Load("../../../../.env")
	if os.Getenv("RUN_DB_TESTS") != "1" {
		os.Exit(0)
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		os.Exit(0)
	}
	if _, err := dbpkg.InitZocDB(context.Background(), dbURL); err != nil {
		os.Exit(0)
	}
	code := m.Run()
	dbpkg.CloseZocDB()
	os.Exit(code)
}

func requirePool(t *testing.T) {
	t.Helper()
	if dbpkg.GetZocPoolOrNil() == nil {
		t.Skip("DATABASE_URL not set; skipping DB test")
	}
}

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano())
}

func TestCreateAndGetFolder(t *testing.T) {
	requirePool(t)
	ctx := context.Background()
	user := uuid.NewString()

	f, err := CreateFolder(ctx, nil, uniqueName("folder-"), user)
	if err != nil {
		t.Fatalf("CreateFolder failed: %v", err)
	}
	if f.FolderID == "" || f.Path == "" {
		t.Fatalf("unexpected folder after create: %+v", f)
	}

	got, err := GetFolder(ctx, f.FolderID, user)
	if err != nil {
		t.Fatalf("GetFolder failed: %v", err)
	}
	if got.Name != f.Name {
		t.Fatalf("unexpected folder: %+v", got)
	}
}

func TestListFolders_ByParent(t *testing.T) {
	requirePool(t)
	ctx := context.Background()
	user := uuid.NewString()

	parent, err := CreateFolder(ctx, nil, uniqueName("parent-"), user)
	if err != nil {
		t.Fatalf("CreateFolder (parent) failed: %v", err)
	}
	child, err := CreateFolder(ctx, &parent.FolderID, uniqueName("child-"), user)
	if err != nil {
		t.Fatalf("CreateFolder (child) failed: %v", err)
	}

	list, err := ListFolders(ctx, &parent.FolderID, user)
	if err != nil {
		t.Fatalf("ListFolders failed: %v", err)
	}
	if len(list) != 1 || list[0].FolderID != child.FolderID {
		t.Fatalf("expected exactly the child folder, got %+v", list)
	}
}

func TestRenameAndMoveFolder(t *testing.T) {
	requirePool(t)
	ctx := context.Background()
	user := uuid.NewString()

	f, err := CreateFolder(ctx, nil, uniqueName("folder-"), user)
	if err != nil {
		t.Fatalf("CreateFolder failed: %v", err)
	}
	other, err := CreateFolder(ctx, nil, uniqueName("other-"), user)
	if err != nil {
		t.Fatalf("CreateFolder (other) failed: %v", err)
	}

	if err := RenameFolder(ctx, f.FolderID, "renamed", user); err != nil {
		t.Fatalf("RenameFolder failed: %v", err)
	}
	if err := MoveFolder(ctx, f.FolderID, &other.FolderID, user); err != nil {
		t.Fatalf("MoveFolder failed: %v", err)
	}

	got, err := GetFolder(ctx, f.FolderID, user)
	if err != nil {
		t.Fatalf("GetFolder failed: %v", err)
	}
	if got.Name != "renamed" || got.ParentID == nil || *got.ParentID != other.FolderID {
		t.Fatalf("rename/move did not persist: %+v", got)
	}
}

func TestSoftDeleteRestoreAndTrashFolder(t *testing.T) {
	requirePool(t)
	ctx := context.Background()
	user := uuid.NewString()

	f, err := CreateFolder(ctx, nil, uniqueName("folder-"), user)
	if err != nil {
		t.Fatalf("CreateFolder failed: %v", err)
	}

	if err := SoftDeleteFolder(ctx, f.FolderID, user); err != nil {
		t.Fatalf("SoftDeleteFolder failed: %v", err)
	}

	trashed, err := ListTrashedFolders(ctx, user)
	if err != nil {
		t.Fatalf("ListTrashedFolders failed: %v", err)
	}
	found := false
	for _, tf := range trashed {
		if tf.FolderID == f.FolderID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected soft-deleted folder in trash, got %+v", trashed)
	}

	if err := RestoreFolder(ctx, f.FolderID, user); err != nil {
		t.Fatalf("RestoreFolder failed: %v", err)
	}
	got, err := GetFolder(ctx, f.FolderID, user)
	if err != nil {
		t.Fatalf("GetFolder after restore failed: %v", err)
	}
	if got.DeletedAt != nil {
		t.Fatalf("expected folder restored (deleted_at nil), got %+v", got)
	}

	if err := DeleteFolder(ctx, f.FolderID, user); err != nil {
		t.Fatalf("DeleteFolder (hard) failed: %v", err)
	}
	if _, err := GetFolder(ctx, f.FolderID, user); err == nil {
		t.Fatal("expected error getting permanently-deleted folder")
	}
}
