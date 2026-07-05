package handlers

import (
	"net/http"

	dbdocument "zoc/src/internal/db/document"
	dbtag "zoc/src/internal/db/tag"
	modelsdoc "zoc/src/internal/models/document"
	models "zoc/src/internal/models/tag"
	"zoc/src/internal/utils"

	"github.com/go-chi/chi/v5"
)

type createTagRequest struct {
	Name string `json:"name"`
}

func CreateTagHandler(w http.ResponseWriter, r *http.Request) {
	var req createTagRequest
	if err := utils.ReadJSON(r, &req); err != nil || req.Name == "" {
		utils.WriteError(w, http.StatusBadRequest, "name is required")
		return
	}
	userID, _ := r.Context().Value("user_id").(string)
	t, err := dbtag.CreateTag(r.Context(), req.Name, userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to create tag: "+err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusCreated, t)
}

func ListTagsHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value("user_id").(string)
	list, err := dbtag.ListTags(r.Context(), userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to list tags: "+err.Error())
		return
	}
	if list == nil {
		list = []models.Tag{}
	}
	utils.WriteJSON(w, http.StatusOK, list)
}

func DeleteTagHandler(w http.ResponseWriter, r *http.Request) {
	tagID := chi.URLParam(r, "tagId")
	if err := dbtag.DeleteTag(r.Context(), tagID); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to delete tag: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func AddDocumentTagHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	tagID := chi.URLParam(r, "tagId")
	if err := dbtag.AddDocumentTag(r.Context(), documentID, tagID); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to tag document: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func RemoveDocumentTagHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	tagID := chi.URLParam(r, "tagId")
	if err := dbtag.RemoveDocumentTag(r.Context(), documentID, tagID); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to untag document: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func ListDocumentTagsHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	list, err := dbtag.ListDocumentTags(r.Context(), documentID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to list document tags: "+err.Error())
		return
	}
	if list == nil {
		list = []models.Tag{}
	}
	utils.WriteJSON(w, http.StatusOK, list)
}

func RecentDocumentsHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value("user_id").(string)
	ids, err := dbtag.ListRecentDocumentIDs(r.Context(), userID, 20)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to list recent documents: "+err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusOK, resolveDocs(r, ids))
}

func StarDocumentHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	userID, _ := r.Context().Value("user_id").(string)
	if err := dbtag.StarDocument(r.Context(), documentID, userID); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to star document: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func UnstarDocumentHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	userID, _ := r.Context().Value("user_id").(string)
	if err := dbtag.UnstarDocument(r.Context(), documentID, userID); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to unstar document: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func ListStarredDocumentsHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value("user_id").(string)
	ids, err := dbtag.ListStarredDocumentIDs(r.Context(), userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to list starred documents: "+err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusOK, resolveDocs(r, ids))
}

func resolveDocs(r *http.Request, ids []string) []modelsdoc.Document {
	docs := []modelsdoc.Document{}
	for _, id := range ids {
		d, err := dbdocument.GetDocument(r.Context(), id)
		if err == nil && d.DeletedAt == nil {
			docs = append(docs, *d)
		}
	}
	return docs
}
