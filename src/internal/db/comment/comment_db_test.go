package db

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	dbpkg "zoc/src/internal/db"
	dbdocument "zoc/src/internal/db/document"

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

func seedDocumentID(t *testing.T, user string) string {
	t.Helper()
	d, err := dbdocument.CreateDocument(context.Background(), nil, uniqueName("doc-"), "application/x-zef-doc", 0, "", "", user)
	if err != nil {
		t.Fatalf("CreateDocument (seed) failed: %v", err)
	}
	return d.DocumentID
}

func TestCreateAndListComments(t *testing.T) {
	requirePool(t)
	user := uuid.NewString()
	docID := seedDocumentID(t, user)

	c, err := CreateComment(context.Background(), docID, nil, nil, 0, 10, "hello world", user)
	if err != nil {
		t.Fatalf("CreateComment failed: %v", err)
	}
	if c.CommentID == "" || c.Body != "hello world" {
		t.Fatalf("unexpected comment after create: %+v", c)
	}

	list, err := ListComments(context.Background(), docID)
	if err != nil {
		t.Fatalf("ListComments failed: %v", err)
	}
	if len(list) != 1 || list[0].CommentID != c.CommentID {
		t.Fatalf("expected exactly the seeded comment, got %+v", list)
	}
}

func TestResolveAndUnresolveComment(t *testing.T) {
	requirePool(t)
	user := uuid.NewString()
	docID := seedDocumentID(t, user)

	c, err := CreateComment(context.Background(), docID, nil, nil, 0, 5, "resolve me", user)
	if err != nil {
		t.Fatalf("CreateComment failed: %v", err)
	}

	resolved, err := ResolveComment(context.Background(), c.CommentID, true, user)
	if err != nil {
		t.Fatalf("ResolveComment(true) failed: %v", err)
	}
	if !resolved.Resolved || resolved.ResolvedBy == nil || *resolved.ResolvedBy != user {
		t.Fatalf("expected comment resolved by %s, got %+v", user, resolved)
	}

	unresolved, err := ResolveComment(context.Background(), c.CommentID, false, user)
	if err != nil {
		t.Fatalf("ResolveComment(false) failed: %v", err)
	}
	if unresolved.Resolved || unresolved.ResolvedBy != nil {
		t.Fatalf("expected comment unresolved, got %+v", unresolved)
	}
}

func TestDeleteComment(t *testing.T) {
	requirePool(t)
	user := uuid.NewString()
	docID := seedDocumentID(t, user)

	c, err := CreateComment(context.Background(), docID, nil, nil, 0, 5, "to delete", user)
	if err != nil {
		t.Fatalf("CreateComment failed: %v", err)
	}
	if err := DeleteComment(context.Background(), c.CommentID); err != nil {
		t.Fatalf("DeleteComment failed: %v", err)
	}

	list, err := ListComments(context.Background(), docID)
	if err != nil {
		t.Fatalf("ListComments failed: %v", err)
	}
	for _, item := range list {
		if item.CommentID == c.CommentID {
			t.Fatalf("expected comment removed, but still present: %+v", item)
		}
	}
}
