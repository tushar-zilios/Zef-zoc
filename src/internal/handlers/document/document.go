package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	dbactivity "zoc/src/internal/db/activity"
	dbchunk "zoc/src/internal/db/chunk"
	dbdocument "zoc/src/internal/db/document"
	dblink "zoc/src/internal/db/link"
	dbtag "zoc/src/internal/db/tag"
	modelschunk "zoc/src/internal/models/chunk"
	models "zoc/src/internal/models/document"
	"zoc/src/internal/storage"
	"zoc/src/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// RichTextMimeType marks a document as chunk-based rich text, stored in
// zoc_document_chunks rather than as a GCS blob.
const RichTextMimeType = "application/x-zef-doc"

type createDocumentRequest struct {
	FolderID   *string `json:"folder_id,omitempty"`
	Name       string  `json:"name"`
	Type       string  `json:"type,omitempty"`        // "rich_text" or omitted for a plain file
	TemplateID string  `json:"template_id,omitempty"` // clone this template's content instead of seeding empty
}

func CreateDocumentHandler(w http.ResponseWriter, r *http.Request) {
	var req createDocumentRequest
	if err := utils.ReadJSON(r, &req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Name == "" {
		utils.WriteError(w, http.StatusBadRequest, "name is required")
		return
	}

	userID, _ := r.Context().Value("user_id").(string)

	if req.TemplateID != "" {
		createFromTemplate(w, r, req, userID)
		return
	}

	if req.Type == "rich_text" {
		createRichTextDocument(w, r, req, userID)
		return
	}

	client := storage.GetClient()
	if client == nil {
		utils.WriteError(w, http.StatusServiceUnavailable, "Storage not configured")
		return
	}

	objectName := "documents/" + uuid.NewString() + ".txt"
	storageKey, err := client.UploadObject(objectName, []byte{}, "text/plain")
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to create storage object: "+err.Error())
		return
	}

	d, err := dbdocument.CreateDocument(r.Context(), req.FolderID, req.Name, "text/plain", 0, storageKey, "", userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to create document: "+err.Error())
		return
	}
	_ = dbactivity.LogActivity(r.Context(), d.DocumentID, userID, "created", nil)
	utils.WriteJSON(w, http.StatusCreated, d)
}

func createRichTextDocument(w http.ResponseWriter, r *http.Request, req createDocumentRequest, userID string) {
	d, err := dbdocument.CreateDocument(r.Context(), req.FolderID, req.Name, RichTextMimeType, 0, "", "", userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to create document: "+err.Error())
		return
	}

	emptyDoc := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph"}]}`)
	if _, err := dbchunk.ReplaceChunks(r.Context(), d.DocumentID, []modelschunk.ChunkInput{
		{ChunkIndex: 0, ChunkType: "text", Content: emptyDoc},
	}); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to seed document content: "+err.Error())
		return
	}

	_ = dbactivity.LogActivity(r.Context(), d.DocumentID, userID, "created", nil)
	utils.WriteJSON(w, http.StatusCreated, d)
}

func createFromTemplate(w http.ResponseWriter, r *http.Request, req createDocumentRequest, userID string) {
	tmpl, err := dbdocument.GetDocument(r.Context(), req.TemplateID)
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, "Template not found")
		return
	}
	if !tmpl.IsTemplate {
		utils.WriteError(w, http.StatusBadRequest, "document is not a template")
		return
	}

	d, err := dbdocument.CreateDocument(r.Context(), req.FolderID, req.Name, RichTextMimeType, 0, "", "", userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to create document: "+err.Error())
		return
	}

	srcChunks, err := dbchunk.ListChunks(r.Context(), tmpl.DocumentID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to read template content: "+err.Error())
		return
	}
	inputs := make([]modelschunk.ChunkInput, len(srcChunks))
	for i, c := range srcChunks {
		inputs[i] = modelschunk.ChunkInput{ChunkIndex: c.ChunkIndex, ChunkType: c.ChunkType, Content: c.Content}
	}
	if _, err := dbchunk.ReplaceChunks(r.Context(), d.DocumentID, inputs); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to clone template content: "+err.Error())
		return
	}

	_ = dbactivity.LogActivity(r.Context(), d.DocumentID, userID, "created", map[string]string{"from_template": tmpl.DocumentID})
	utils.WriteJSON(w, http.StatusCreated, d)
}

func ListDocumentsHandler(w http.ResponseWriter, r *http.Request) {
	var folderID *string
	if v := r.URL.Query().Get("folder_id"); v != "" {
		folderID = &v
	}

	list, err := dbdocument.ListDocuments(r.Context(), folderID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to list documents: "+err.Error())
		return
	}
	if list == nil {
		list = []models.Document{}
	}
	utils.WriteJSON(w, http.StatusOK, list)
}

func GetDocumentHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	d, err := dbdocument.GetDocument(r.Context(), documentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.WriteError(w, http.StatusNotFound, "Document not found")
			return
		}
		utils.WriteError(w, http.StatusInternalServerError, "Failed to get document: "+err.Error())
		return
	}
	if userID, _ := r.Context().Value("user_id").(string); userID != "" {
		_ = dbtag.RecordView(r.Context(), documentID, userID)
	}
	utils.WriteJSON(w, http.StatusOK, d)
}

type updateDocumentRequest struct {
	Name       *string `json:"name,omitempty"`
	FolderID   *string `json:"folder_id,omitempty"`
	StorageKey *string `json:"storage_key,omitempty"`
	Checksum   *string `json:"checksum,omitempty"`
	SizeBytes  *int64  `json:"size_bytes,omitempty"`
}

func UpdateDocumentHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	var req updateDocumentRequest
	if err := utils.ReadJSON(r, &req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	userID, _ := r.Context().Value("user_id").(string)

	d, err := dbdocument.UpdateDocument(r.Context(), documentID, req.Name, req.FolderID, req.StorageKey, req.Checksum, req.SizeBytes, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.WriteError(w, http.StatusNotFound, "Document not found")
			return
		}
		utils.WriteError(w, http.StatusInternalServerError, "Failed to update document: "+err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusOK, d)
}

// requireOwner returns the document if the requester created it, otherwise
// writes a 403 and returns nil. Used for delete/archive/share/template actions.
func requireOwner(w http.ResponseWriter, r *http.Request, documentID string) *models.Document {
	d, err := dbdocument.GetDocument(r.Context(), documentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.WriteError(w, http.StatusNotFound, "Document not found")
			return nil
		}
		utils.WriteError(w, http.StatusInternalServerError, "Failed to load document: "+err.Error())
		return nil
	}
	userID, _ := r.Context().Value("user_id").(string)
	if d.CreatedBy != userID {
		utils.WriteError(w, http.StatusForbidden, "Only the document owner can perform this action")
		return nil
	}
	return d
}

func DeleteDocumentHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	if requireOwner(w, r, documentID) == nil {
		return
	}
	if err := dbdocument.SoftDeleteDocument(r.Context(), documentID); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to delete document: "+err.Error())
		return
	}
	userID, _ := r.Context().Value("user_id").(string)
	_ = dbactivity.LogActivity(r.Context(), documentID, userID, "deleted", nil)
	w.WriteHeader(http.StatusNoContent)
}

func RestoreDocumentHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	if requireOwner(w, r, documentID) == nil {
		return
	}
	if err := dbdocument.RestoreDocument(r.Context(), documentID); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to restore document: "+err.Error())
		return
	}
	userID, _ := r.Context().Value("user_id").(string)
	_ = dbactivity.LogActivity(r.Context(), documentID, userID, "restored", nil)
	d, err := dbdocument.GetDocument(r.Context(), documentID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to fetch restored document: "+err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusOK, d)
}

func ListTrashedDocumentsHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value("user_id").(string)
	list, err := dbdocument.ListTrashedDocuments(r.Context(), userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to list trash: "+err.Error())
		return
	}
	if list == nil {
		list = []models.Document{}
	}
	utils.WriteJSON(w, http.StatusOK, list)
}

func ArchiveDocumentHandler(w http.ResponseWriter, r *http.Request) {
	setArchived(w, r, true)
}

func UnarchiveDocumentHandler(w http.ResponseWriter, r *http.Request) {
	setArchived(w, r, false)
}

func setArchived(w http.ResponseWriter, r *http.Request, archived bool) {
	documentID := chi.URLParam(r, "id")
	if requireOwner(w, r, documentID) == nil {
		return
	}
	d, err := dbdocument.SetArchived(r.Context(), documentID, archived)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to update archive state: "+err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusOK, d)
}

func DuplicateDocumentHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	userID, _ := r.Context().Value("user_id").(string)
	d, err := dbdocument.DuplicateDocument(r.Context(), documentID, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.WriteError(w, http.StatusNotFound, "Document not found")
			return
		}
		utils.WriteError(w, http.StatusInternalServerError, "Failed to duplicate document: "+err.Error())
		return
	}
	_ = dbactivity.LogActivity(r.Context(), d.DocumentID, userID, "created", map[string]string{"duplicated_from": documentID})
	utils.WriteJSON(w, http.StatusCreated, d)
}

type bulkMoveRequest struct {
	DocumentIDs []string `json:"document_ids"`
	FolderID    *string  `json:"folder_id"`
}

func BulkMoveDocumentsHandler(w http.ResponseWriter, r *http.Request) {
	var req bulkMoveRequest
	if err := utils.ReadJSON(r, &req); err != nil || len(req.DocumentIDs) == 0 {
		utils.WriteError(w, http.StatusBadRequest, "document_ids is required")
		return
	}
	if err := dbdocument.BulkMoveDocuments(r.Context(), req.DocumentIDs, req.FolderID); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to move documents: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func ListDocumentVersionsHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	list, err := dbdocument.ListDocumentVersions(r.Context(), documentID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to list versions: "+err.Error())
		return
	}
	if list == nil {
		list = []models.DocumentVersion{}
	}
	utils.WriteJSON(w, http.StatusOK, list)
}

// RestoreVersionHandler replays a prior chunk snapshot as the live content,
// recording the restore itself as a brand-new version (non-destructive rollback).
func RestoreVersionHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	version, err := strconv.Atoi(chi.URLParam(r, "version"))
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid version")
		return
	}

	v, err := dbdocument.GetDocumentVersion(r.Context(), documentID, version)
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, "Version not found")
		return
	}
	if len(v.ChunksSnapshot) == 0 {
		utils.WriteError(w, http.StatusBadRequest, "This version has no chunk snapshot to restore")
		return
	}

	var chunks []modelschunk.DocumentChunk
	if err := json.Unmarshal(v.ChunksSnapshot, &chunks); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to parse version snapshot: "+err.Error())
		return
	}
	inputs := make([]modelschunk.ChunkInput, len(chunks))
	for i, c := range chunks {
		inputs[i] = modelschunk.ChunkInput{ChunkIndex: c.ChunkIndex, ChunkType: c.ChunkType, Content: c.Content}
	}

	newChunks, err := dbchunk.ReplaceChunks(r.Context(), documentID, inputs)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to restore version: "+err.Error())
		return
	}

	userID, _ := r.Context().Value("user_id").(string)
	if chunksJSON, err := json.Marshal(newChunks); err == nil {
		_, _ = dbdocument.SnapshotChunksVersion(r.Context(), documentID, chunksJSON, userID)
	}
	_ = dbactivity.LogActivity(r.Context(), documentID, userID, "restored", map[string]int{"from_version": version})

	utils.WriteJSON(w, http.StatusOK, map[string]any{"chunks": newChunks})
}

// DiffVersionsHandler returns a coarse chunk-level diff: which chunk indexes
// were added, removed, or changed between two versions.
func DiffVersionsHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	v1, err := strconv.Atoi(chi.URLParam(r, "v1"))
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid v1")
		return
	}
	v2, err := strconv.Atoi(chi.URLParam(r, "v2"))
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid v2")
		return
	}

	a, err := dbdocument.GetDocumentVersion(r.Context(), documentID, v1)
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, "v1 not found")
		return
	}
	b, err := dbdocument.GetDocumentVersion(r.Context(), documentID, v2)
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, "v2 not found")
		return
	}

	var chunksA, chunksB []modelschunk.DocumentChunk
	_ = json.Unmarshal(a.ChunksSnapshot, &chunksA)
	_ = json.Unmarshal(b.ChunksSnapshot, &chunksB)

	byIndexA := map[int]modelschunk.DocumentChunk{}
	for _, c := range chunksA {
		byIndexA[c.ChunkIndex] = c
	}
	byIndexB := map[int]modelschunk.DocumentChunk{}
	for _, c := range chunksB {
		byIndexB[c.ChunkIndex] = c
	}

	var added, removed, changed []int
	for idx, cb := range byIndexB {
		ca, ok := byIndexA[idx]
		if !ok {
			added = append(added, idx)
		} else if string(ca.Content) != string(cb.Content) {
			changed = append(changed, idx)
		}
	}
	for idx := range byIndexA {
		if _, ok := byIndexB[idx]; !ok {
			removed = append(removed, idx)
		}
	}

	utils.WriteJSON(w, http.StatusOK, map[string]any{"added": added, "removed": removed, "changed": changed})
}

func ListTemplatesHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value("user_id").(string)
	list, err := dbdocument.ListTemplates(r.Context(), userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to list templates: "+err.Error())
		return
	}
	if list == nil {
		list = []models.Document{}
	}
	utils.WriteJSON(w, http.StatusOK, list)
}

func MarkTemplateHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	if requireOwner(w, r, documentID) == nil {
		return
	}
	if err := dbdocument.SetIsTemplate(r.Context(), documentID, true); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to mark as template: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func UnmarkTemplateHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	if requireOwner(w, r, documentID) == nil {
		return
	}
	if err := dbdocument.SetIsTemplate(r.Context(), documentID, false); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to unmark template: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func BacklinksHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	ids, err := dblink.ListBacklinks(r.Context(), documentID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to list backlinks: "+err.Error())
		return
	}
	if ids == nil {
		ids = []string{}
	}
	utils.WriteJSON(w, http.StatusOK, map[string]any{"document_ids": ids})
}

func TOCHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	chunks, err := dbchunk.ListChunks(r.Context(), documentID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to load content: "+err.Error())
		return
	}
	type tocEntry struct {
		ChunkID    string `json:"chunk_id"`
		ChunkIndex int    `json:"chunk_index"`
		Text       string `json:"text"`
	}
	entries := []tocEntry{}
	for _, c := range chunks {
		if c.ChunkType == "heading" {
			entries = append(entries, tocEntry{ChunkID: c.ChunkID, ChunkIndex: c.ChunkIndex, Text: modelschunk.ExtractText(c.Content)})
		}
	}
	utils.WriteJSON(w, http.StatusOK, entries)
}
