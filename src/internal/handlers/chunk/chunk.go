package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	dbactivity "zoc/src/internal/db/activity"
	dbchunk "zoc/src/internal/db/chunk"
	dbdocument "zoc/src/internal/db/document"
	dblink "zoc/src/internal/db/link"
	"zoc/src/internal/logger"
	models "zoc/src/internal/models/chunk"
	"zoc/src/internal/utils"

	"github.com/go-chi/chi/v5"
)

type saveContentRequest struct {
	Chunks []models.ChunkInput `json:"chunks"`
}

func GetDocumentContentHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	chunks, err := dbchunk.ListChunks(r.Context(), documentID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to list chunks: "+err.Error())
		return
	}
	if chunks == nil {
		chunks = []models.DocumentChunk{}
	}
	utils.WriteJSON(w, http.StatusOK, map[string]any{"chunks": chunks})
}

func SaveDocumentContentHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	var req saveContentRequest
	if err := utils.ReadJSON(r, &req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	chunks, err := dbchunk.ReplaceChunks(r.Context(), documentID, req.Chunks)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to save chunks: "+err.Error())
		return
	}
	if chunks == nil {
		chunks = []models.DocumentChunk{}
	}

	userID, _ := r.Context().Value("user_id").(string)
	applyContentSideEffects(r, documentID, chunks, userID)

	utils.WriteJSON(w, http.StatusOK, map[string]any{"chunks": chunks})
}

type ydocResponse struct {
	YDocState string `json:"ydoc_state,omitempty"`
}

// InternalGetYDocHandler is called by Zef-zoc-collab's onLoadDocument hook.
func InternalGetYDocHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	state, err := dbchunk.GetYDocState(r.Context(), documentID)
	if err != nil {
		// No stored state yet is a normal case for a doc never opened in collab mode.
		utils.WriteJSON(w, http.StatusOK, ydocResponse{})
		return
	}
	utils.WriteJSON(w, http.StatusOK, ydocResponse{YDocState: base64.StdEncoding.EncodeToString(state)})
}

type internalSaveYDocRequest struct {
	YDocState string              `json:"ydoc_state"`
	Chunks    []models.ChunkInput `json:"chunks"`
	UpdatedBy string              `json:"updated_by"`
}

// InternalSaveYDocHandler is called by Zef-zoc-collab's debounced onStoreDocument hook.
func InternalSaveYDocHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	var req internalSaveYDocRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	state, err := base64.StdEncoding.DecodeString(req.YDocState)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid ydoc_state encoding")
		return
	}

	if err := dbchunk.UpsertYDocState(r.Context(), documentID, state); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to store ydoc state: "+err.Error())
		return
	}

	chunks, err := dbchunk.ReplaceChunks(r.Context(), documentID, req.Chunks)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to flatten chunks: "+err.Error())
		return
	}

	applyContentSideEffects(r, documentID, chunks, req.UpdatedBy)

	w.WriteHeader(http.StatusNoContent)
}

// applyContentSideEffects rolls up plain text for search, snapshots a version
// for restore/diff, refreshes backlinks parsed from @doc-link mentions, and
// logs an activity entry. Best-effort: failures here don't fail the save.
func applyContentSideEffects(r *http.Request, documentID string, chunks []models.DocumentChunk, userID string) {
	ctx := r.Context()

	var texts []string
	var mentions []string
	for _, c := range chunks {
		if text := models.ExtractText(c.Content); text != "" {
			texts = append(texts, text)
		}
		mentions = append(mentions, models.ExtractMentionIDs(c.Content)...)
	}
	_ = dbdocument.UpdateSearchText(ctx, documentID, strings.Join(texts, " "))
	_ = dblink.ReplaceOutgoingLinks(ctx, documentID, mentions)

	if chunksJSON, err := json.Marshal(chunks); err == nil {
		if _, err := dbdocument.SnapshotChunksVersion(ctx, documentID, chunksJSON, userID); err != nil {
			logger.LogHandler("SnapshotChunksVersion failed: %v", err)
		}
	}

	_ = dbactivity.LogActivity(ctx, documentID, userID, "edited", nil)
}
