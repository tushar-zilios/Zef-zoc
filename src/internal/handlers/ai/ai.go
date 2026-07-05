package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"zoc/src/internal/clients/gemini"
	dbchunk "zoc/src/internal/db/chunk"
	modelschunk "zoc/src/internal/models/chunk"
	"zoc/src/internal/utils"

	"github.com/go-chi/chi/v5"
)

func documentText(r *http.Request, documentID string) (string, error) {
	chunks, err := dbchunk.ListChunks(r.Context(), documentID)
	if err != nil {
		return "", err
	}
	var texts []string
	for _, c := range chunks {
		if t := modelschunk.ExtractText(c.Content); t != "" {
			texts = append(texts, t)
		}
	}
	return strings.Join(texts, "\n"), nil
}

func SummarizeDocumentHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	text, err := documentText(r, documentID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to load content: "+err.Error())
		return
	}
	if text == "" {
		utils.WriteJSON(w, http.StatusOK, map[string]string{"summary": ""})
		return
	}

	summary, err := gemini.GenerateContent(r.Context(), "You summarize documents concisely.", fmt.Sprintf("Summarize the following document in 3-5 sentences:\n\n%s", text))
	if err != nil {
		utils.WriteError(w, http.StatusServiceUnavailable, "AI summarization unavailable: "+err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]string{"summary": summary})
}

type askRequest struct {
	Question string `json:"question"`
}

func AskDocumentHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	var req askRequest
	if err := utils.ReadJSON(r, &req); err != nil || req.Question == "" {
		utils.WriteError(w, http.StatusBadRequest, "question is required")
		return
	}

	text, err := documentText(r, documentID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to load content: "+err.Error())
		return
	}

	systemPrompt := fmt.Sprintf("Answer questions about the following document.\n\n%s", text)
	answer, err := gemini.GenerateContent(r.Context(), systemPrompt, req.Question)
	if err != nil {
		utils.WriteError(w, http.StatusServiceUnavailable, "AI Q&A unavailable: "+err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]string{"answer": answer})
}
