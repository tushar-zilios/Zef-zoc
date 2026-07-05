package handlers

import (
	"net/http"

	dbactivity "zoc/src/internal/db/activity"
	models "zoc/src/internal/models/activity"
	"zoc/src/internal/utils"

	"github.com/go-chi/chi/v5"
)

func ListActivityHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	list, err := dbactivity.ListActivity(r.Context(), documentID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to list activity: "+err.Error())
		return
	}
	if list == nil {
		list = []models.Activity{}
	}
	utils.WriteJSON(w, http.StatusOK, list)
}
