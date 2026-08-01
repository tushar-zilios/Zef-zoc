package db

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	dbpkg "zoc/src/internal/db"
	dbdocument "zoc/src/internal/db/document"
	models "zoc/src/internal/models/chunk"

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

func TestReplaceChunksAndListChunks(t *testing.T) {
	requirePool(t)
	user := uuid.NewString()
	docID := seedDocumentID(t, user)

	content, _ := json.Marshal(map[string]string{"text": "hello"})
	chunks, err := ReplaceChunks(context.Background(), docID, []models.ChunkInput{
		{ChunkIndex: 0, ChunkType: "text", Content: content},
	})
	if err != nil {
		t.Fatalf("ReplaceChunks failed: %v", err)
	}
	if len(chunks) != 1 || chunks[0].ChunkType != "text" {
		t.Fatalf("unexpected chunks after replace: %+v", chunks)
	}

	list, err := ListChunks(context.Background(), docID)
	if err != nil {
		t.Fatalf("ListChunks failed: %v", err)
	}
	if len(list) != 1 || list[0].ChunkIndex != 0 {
		t.Fatalf("expected exactly one chunk, got %+v", list)
	}
}

func TestReplaceChunks_DefaultsChunkType(t *testing.T) {
	requirePool(t)
	user := uuid.NewString()
	docID := seedDocumentID(t, user)

	content, _ := json.Marshal(map[string]string{"text": "no type"})
	chunks, err := ReplaceChunks(context.Background(), docID, []models.ChunkInput{
		{ChunkIndex: 0, Content: content},
	})
	if err != nil {
		t.Fatalf("ReplaceChunks failed: %v", err)
	}
	if len(chunks) != 1 || chunks[0].ChunkType != "text" {
		t.Fatalf("expected default chunk_type 'text', got %+v", chunks)
	}
}

func TestReplaceChunks_ReplacesPreviousSet(t *testing.T) {
	requirePool(t)
	user := uuid.NewString()
	docID := seedDocumentID(t, user)

	c1, _ := json.Marshal(map[string]string{"text": "first"})
	if _, err := ReplaceChunks(context.Background(), docID, []models.ChunkInput{{ChunkIndex: 0, ChunkType: "text", Content: c1}}); err != nil {
		t.Fatalf("ReplaceChunks (first) failed: %v", err)
	}

	c2, _ := json.Marshal(map[string]string{"text": "second"})
	if _, err := ReplaceChunks(context.Background(), docID, []models.ChunkInput{{ChunkIndex: 0, ChunkType: "text", Content: c2}}); err != nil {
		t.Fatalf("ReplaceChunks (second) failed: %v", err)
	}

	list, err := ListChunks(context.Background(), docID)
	if err != nil {
		t.Fatalf("ListChunks failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected old chunks fully replaced, got %d chunks", len(list))
	}
}

func TestUpsertAndGetYDocState(t *testing.T) {
	requirePool(t)
	user := uuid.NewString()
	docID := seedDocumentID(t, user)

	state := []byte{1, 2, 3, 4}
	if err := UpsertYDocState(context.Background(), docID, state); err != nil {
		t.Fatalf("UpsertYDocState (insert) failed: %v", err)
	}
	got, err := GetYDocState(context.Background(), docID)
	if err != nil {
		t.Fatalf("GetYDocState failed: %v", err)
	}
	if string(got) != string(state) {
		t.Fatalf("expected ydoc state %v, got %v", state, got)
	}

	updated := []byte{9, 9, 9}
	if err := UpsertYDocState(context.Background(), docID, updated); err != nil {
		t.Fatalf("UpsertYDocState (update) failed: %v", err)
	}
	got, err = GetYDocState(context.Background(), docID)
	if err != nil {
		t.Fatalf("GetYDocState (after update) failed: %v", err)
	}
	if string(got) != string(updated) {
		t.Fatalf("expected updated ydoc state %v, got %v", updated, got)
	}
}

func TestGetYDocState_NotFound(t *testing.T) {
	requirePool(t)
	_, err := GetYDocState(context.Background(), "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("expected error for nonexistent ydoc state")
	}
}
