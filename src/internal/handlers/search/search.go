package handlers

import (
	"net/http"

	dbdocument "zoc/src/internal/db/document"
	models "zoc/src/internal/models/document"
	"zoc/src/internal/utils"
)

func SearchHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		utils.WriteError(w, http.StatusBadRequest, "q is required")
		return
	}
	folderID := r.URL.Query().Get("folder")
	tagID := r.URL.Query().Get("tag")

	list, err := dbdocument.Search(r.Context(), q, folderID, tagID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Search failed: "+err.Error())
		return
	}
	if list == nil {
		list = []models.Document{}
	}
	utils.WriteJSON(w, http.StatusOK, list)
}
