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

	d, err := dbdocument.GetDocument(r.Context(), documentID, userID)
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

type shareWithUserRequest struct {
	UserID     string `json:"user_id"`
	Permission string `json:"permission"`
}

// ShareDocumentWithUserHandler grants a specific user standing access to a document.
func ShareDocumentWithUserHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	userID, _ := r.Context().Value("user_id").(string)

	d, err := dbdocument.GetDocument(r.Context(), documentID, userID)
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, "Document not found")
		return
	}
	if d.CreatedBy != userID {
		utils.WriteError(w, http.StatusForbidden, "Only the document owner can share this document")
		return
	}

	var req shareWithUserRequest
	if err := utils.ReadJSON(r, &req); err != nil || req.UserID == "" {
		utils.WriteError(w, http.StatusBadRequest, "user_id is required")
		return
	}
	permission := req.Permission
	if permission == "" {
		permission = "view"
	}

	s, err := dbshare.AddDocumentShare(r.Context(), documentID, req.UserID, permission, userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to share document: "+err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusCreated, s)
}

// ListDocumentSharesHandler lists the users a document has been shared with.
func ListDocumentSharesHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	list, err := dbshare.ListDocumentShares(r.Context(), documentID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to list document shares: "+err.Error())
		return
	}
	if list == nil {
		list = []models.DocumentShare{}
	}
	utils.WriteJSON(w, http.StatusOK, list)
}

// RemoveDocumentShareHandler revokes a specific user's standing access to a document.
func RemoveDocumentShareHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	targetUserID := chi.URLParam(r, "userId")
	requesterID, _ := r.Context().Value("user_id").(string)

	d, err := dbdocument.GetDocument(r.Context(), documentID, requesterID)
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, "Document not found")
		return
	}
	if d.CreatedBy != requesterID {
		utils.WriteError(w, http.StatusForbidden, "Only the document owner can revoke access")
		return
	}

	if err := dbshare.RemoveDocumentShare(r.Context(), documentID, targetUserID); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to revoke document share: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CanAccessDocument reports whether userID may access documentID, either as
// owner or via a document_shares grant, and if so, at what permission level
// ("owner" for the creator, else the granted permission string).
func CanAccessDocument(r *http.Request, documentID, userID string) (bool, string) {
	d, err := dbdocument.GetDocumentByID(r.Context(), documentID)
	if err != nil {
		return false, ""
	}
	if d.CreatedBy == userID {
		return true, "owner"
	}
	permission, err := dbshare.GetUserPermission(r.Context(), documentID, userID)
	if err != nil {
		return false, ""
	}
	return true, permission
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

	d, err := dbdocument.GetDocument(r.Context(), s.DocumentID, s.CreatedBy)
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
