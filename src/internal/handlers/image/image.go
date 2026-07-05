package handlers

import (
	"io"
	"net/http"

	"zoc/src/internal/storage"
	"zoc/src/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxImageBytes = 200 << 20 // 200MB (also covers video uploads)

func UploadImageHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")

	client := storage.GetClient()
	if client == nil {
		utils.WriteError(w, http.StatusServiceUnavailable, "Storage not configured")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxImageBytes)
	if err := r.ParseMultipartForm(maxImageBytes); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid or too-large multipart upload")
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Missing image file field")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	content, err := io.ReadAll(file)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to read upload")
		return
	}

	ext := ""
	if idx := lastDotIndex(header.Filename); idx >= 0 {
		ext = header.Filename[idx:]
	}
	objectName := "images/" + documentID + "/" + uuid.NewString() + ext

	storageKey, err := client.UploadObject(objectName, content, contentType)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to upload image: "+err.Error())
		return
	}

	utils.WriteJSON(w, http.StatusCreated, map[string]string{
		"url":         client.PublicURL(objectName),
		"storage_key": storageKey,
	})
}

func lastDotIndex(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return i
		}
	}
	return -1
}
