package db

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	dbpkg "zoc/src/internal/db"
	models "zoc/src/internal/models/document"

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

func seedDocument(t *testing.T, user string) *models.Document {
	t.Helper()
	d, err := CreateDocument(context.Background(), nil, uniqueName("doc-"), "application/x-zef-doc", 0, "", "", user)
	if err != nil {
		t.Fatalf("CreateDocument failed: %v", err)
	}
	return d
}

func TestCreateAndGetDocument(t *testing.T) {
	requirePool(t)
	user := uuid.NewString()
	d := seedDocument(t, user)
	if d.DocumentID == "" || d.Version != 1 {
		t.Fatalf("unexpected document after create: %+v", d)
	}

	got, err := GetDocument(context.Background(), d.DocumentID, user)
	if err != nil {
		t.Fatalf("GetDocument failed: %v", err)
	}
	if got.Name != d.Name {
		t.Fatalf("unexpected document: %+v", got)
	}
}

func TestListDocuments_ByFolder(t *testing.T) {
	requirePool(t)
	user := uuid.NewString()
	d := seedDocument(t, user)

	list, err := ListDocuments(context.Background(), nil, user)
	if err != nil {
		t.Fatalf("ListDocuments failed: %v", err)
	}
	found := false
	for _, item := range list {
		if item.DocumentID == d.DocumentID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected seeded document in list, got %+v", list)
	}
}

func TestUpdateDocument_BumpsVersionOnStorageKeyChange(t *testing.T) {
	requirePool(t)
	user := uuid.NewString()
	d := seedDocument(t, user)

	newName := "renamed-doc"
	newKey := "storage/key/1"
	newSum := "checksum1"
	var size int64 = 42
	got, err := UpdateDocument(context.Background(), d.DocumentID, &newName, nil, &newKey, &newSum, &size, user)
	if err != nil {
		t.Fatalf("UpdateDocument failed: %v", err)
	}
	if got.Name != newName || got.Version != d.Version+1 || got.StorageKey != newKey {
		t.Fatalf("unexpected document after update: %+v", got)
	}

	versions, err := ListDocumentVersions(context.Background(), d.DocumentID, user)
	if err != nil {
		t.Fatalf("ListDocumentVersions failed: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions after bump, got %d", len(versions))
	}
}

func TestSoftDeleteAndRestoreDocument(t *testing.T) {
	requirePool(t)
	user := uuid.NewString()
	d := seedDocument(t, user)

	if err := SoftDeleteDocument(context.Background(), d.DocumentID, user); err != nil {
		t.Fatalf("SoftDeleteDocument failed: %v", err)
	}
	trashed, err := ListTrashedDocuments(context.Background(), user)
	if err != nil {
		t.Fatalf("ListTrashedDocuments failed: %v", err)
	}
	found := false
	for _, td := range trashed {
		if td.DocumentID == d.DocumentID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected soft-deleted document in trash, got %+v", trashed)
	}

	if err := RestoreDocument(context.Background(), d.DocumentID, user); err != nil {
		t.Fatalf("RestoreDocument failed: %v", err)
	}
	got, err := GetDocument(context.Background(), d.DocumentID, user)
	if err != nil {
		t.Fatalf("GetDocument after restore failed: %v", err)
	}
	if got.DeletedAt != nil {
		t.Fatalf("expected document restored, got %+v", got)
	}
}

func TestSetArchived(t *testing.T) {
	requirePool(t)
	user := uuid.NewString()
	d := seedDocument(t, user)

	got, err := SetArchived(context.Background(), d.DocumentID, user, true)
	if err != nil {
		t.Fatalf("SetArchived(true) failed: %v", err)
	}
	if got.ArchivedAt == nil {
		t.Fatalf("expected archived_at set, got %+v", got)
	}

	got, err = SetArchived(context.Background(), d.DocumentID, user, false)
	if err != nil {
		t.Fatalf("SetArchived(false) failed: %v", err)
	}
	if got.ArchivedAt != nil {
		t.Fatalf("expected archived_at cleared, got %+v", got)
	}
}

func TestDuplicateDocument(t *testing.T) {
	requirePool(t)
	user := uuid.NewString()
	d := seedDocument(t, user)

	dup, err := DuplicateDocument(context.Background(), d.DocumentID, user)
	if err != nil {
		t.Fatalf("DuplicateDocument failed: %v", err)
	}
	if dup.DocumentID == d.DocumentID {
		t.Fatal("expected duplicate to have a new document id")
	}
	if dup.Version != 1 {
		t.Fatalf("expected duplicate version reset to 1, got %d", dup.Version)
	}
}

func TestSetIsTemplateAndListTemplates(t *testing.T) {
	requirePool(t)
	user := uuid.NewString()
	d := seedDocument(t, user)

	if err := SetIsTemplate(context.Background(), d.DocumentID, true, user); err != nil {
		t.Fatalf("SetIsTemplate failed: %v", err)
	}
	list, err := ListTemplates(context.Background(), user)
	if err != nil {
		t.Fatalf("ListTemplates failed: %v", err)
	}
	found := false
	for _, item := range list {
		if item.DocumentID == d.DocumentID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected marked document among templates, got %+v", list)
	}
}

func TestGetDocument_NotFound(t *testing.T) {
	requirePool(t)
	_, err := GetDocument(context.Background(), "00000000-0000-0000-0000-000000000000", "nobody")
	if err == nil {
		t.Fatal("expected error for nonexistent document")
	}
}
