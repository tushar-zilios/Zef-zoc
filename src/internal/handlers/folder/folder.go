package handlers

import (
	"errors"
	"net/http"

	dbfolder "zoc/src/internal/db/folder"
	models "zoc/src/internal/models/folder"
	"zoc/src/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type createFolderRequest struct {
	Name     string  `json:"name"`
	ParentID *string `json:"parent_id,omitempty"`
}

func CreateFolderHandler(w http.ResponseWriter, r *http.Request) {
	var req createFolderRequest
	if err := utils.ReadJSON(r, &req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Name == "" {
		utils.WriteError(w, http.StatusBadRequest, "name is required")
		return
	}

	userID, _ := r.Context().Value("user_id").(string)

	f, err := dbfolder.CreateFolder(r.Context(), req.ParentID, req.Name, userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to create folder: "+err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusCreated, f)
}

func ListFoldersHandler(w http.ResponseWriter, r *http.Request) {
	var parentID *string
	if v := r.URL.Query().Get("parent_id"); v != "" {
		parentID = &v
	}

	list, err := dbfolder.ListFolders(r.Context(), parentID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to list folders: "+err.Error())
		return
	}
	if list == nil {
		list = []models.Folder{}
	}
	utils.WriteJSON(w, http.StatusOK, list)
}

func GetFolderHandler(w http.ResponseWriter, r *http.Request) {
	folderID := chi.URLParam(r, "id")
	f, err := dbfolder.GetFolder(r.Context(), folderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.WriteError(w, http.StatusNotFound, "Folder not found")
			return
		}
		utils.WriteError(w, http.StatusInternalServerError, "Failed to get folder: "+err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusOK, f)
}

type updateFolderRequest struct {
	Name string `json:"name"`
}

func UpdateFolderHandler(w http.ResponseWriter, r *http.Request) {
	folderID := chi.URLParam(r, "id")
	var req updateFolderRequest
	if err := utils.ReadJSON(r, &req); err != nil || req.Name == "" {
		utils.WriteError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := dbfolder.RenameFolder(r.Context(), folderID, req.Name); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to rename folder: "+err.Error())
		return
	}
	f, err := dbfolder.GetFolder(r.Context(), folderID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to fetch updated folder: "+err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusOK, f)
}

type moveFolderRequest struct {
	ParentID *string `json:"parent_id"`
}

func MoveFolderHandler(w http.ResponseWriter, r *http.Request) {
	folderID := chi.URLParam(r, "id")
	var req moveFolderRequest
	if err := utils.ReadJSON(r, &req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := dbfolder.MoveFolder(r.Context(), folderID, req.ParentID); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to move folder: "+err.Error())
		return
	}
	f, err := dbfolder.GetFolder(r.Context(), folderID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to fetch moved folder: "+err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusOK, f)
}

// DeleteFolderHandler soft-deletes, matching document trash behavior.
func DeleteFolderHandler(w http.ResponseWriter, r *http.Request) {
	folderID := chi.URLParam(r, "id")
	if err := dbfolder.SoftDeleteFolder(r.Context(), folderID); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to delete folder: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func RestoreFolderHandler(w http.ResponseWriter, r *http.Request) {
	folderID := chi.URLParam(r, "id")
	if err := dbfolder.RestoreFolder(r.Context(), folderID); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to restore folder: "+err.Error())
		return
	}
	f, err := dbfolder.GetFolder(r.Context(), folderID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to fetch restored folder: "+err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusOK, f)
}

func ListTrashedFoldersHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value("user_id").(string)
	list, err := dbfolder.ListTrashedFolders(r.Context(), userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to list trash: "+err.Error())
		return
	}
	if list == nil {
		list = []models.Folder{}
	}
	utils.WriteJSON(w, http.StatusOK, list)
}
