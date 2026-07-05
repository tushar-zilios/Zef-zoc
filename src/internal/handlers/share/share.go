package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	dbchunk "zoc/src/internal/db/chunk"
	dbdocument "zoc/src/internal/db/document"
	dbshare "zoc/src/internal/db/share"
	models "zoc/src/internal/models/share"
	"zoc/src/internal/utils"

	"github.com/go-chi/chi/v5"
)

type createShareRequest struct {
	Permission   string `json:"permission"`
	ExpiresInMin *int   `json:"expires_in_minutes,omitempty"`
}

func generateToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func CreateShareLinkHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	userID, _ := r.Context().Value("user_id").(string)

	d, err := dbdocument.GetDocument(r.Context(), documentID)
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, "Document not found")
		return
	}
	if d.CreatedBy != userID {
		utils.WriteError(w, http.StatusForbidden, "Only the document owner can create share links")
		return
	}

	var req createShareRequest
	_ = utils.ReadJSON(r, &req)
	permission := req.Permission
	if permission == "" {
		permission = "view"
	}

	token, err := generateToken()
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	var expiresAt *time.Time
	if req.ExpiresInMin != nil {
		t := time.Now().Add(time.Duration(*req.ExpiresInMin) * time.Minute)
		expiresAt = &t
	}

	s, err := dbshare.CreateShareLink(r.Context(), documentID, token, permission, userID, expiresAt)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to create share link: "+err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusCreated, s)
}

func ListShareLinksHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	list, err := dbshare.ListShareLinks(r.Context(), documentID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to list share links: "+err.Error())
		return
	}
	if list == nil {
		list = []models.ShareLink{}
	}
	utils.WriteJSON(w, http.StatusOK, list)
}

func RevokeShareLinkHandler(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if err := dbshare.RevokeShareLink(r.Context(), token); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to revoke share link: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetSharedContentHandler serves read-only document content for a valid,
// unrevoked, unexpired share token. Unauthenticated (no JWT) by design.
func GetSharedContentHandler(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	s, err := dbshare.GetValidShareLink(r.Context(), token)
	if err != nil {
		utils.WriteError(w, http.StatusForbidden, "Invalid, expired, or revoked share link")
		return
	}

	d, err := dbdocument.GetDocument(r.Context(), s.DocumentID)
	if err != nil || d.DeletedAt != nil {
		utils.WriteError(w, http.StatusNotFound, "Document not found")
		return
	}

	chunks, err := dbchunk.ListChunks(r.Context(), s.DocumentID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to load content: "+err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]any{"document": d, "chunks": chunks, "permission": s.Permission})
}
