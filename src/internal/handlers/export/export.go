package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	dbchunk "zoc/src/internal/db/chunk"
	dbdocument "zoc/src/internal/db/document"
	modelschunk "zoc/src/internal/models/chunk"
	"zoc/src/internal/utils"

	"github.com/go-chi/chi/v5"
)

// ExportMarkdownHandler flattens a document's chunks to plain Markdown.
// Headings become "# text", everything else is a paragraph of extracted text.
func ExportMarkdownHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")

	d, err := dbdocument.GetDocument(r.Context(), documentID)
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, "Document not found")
		return
	}

	chunks, err := dbchunk.ListChunks(r.Context(), documentID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to load content: "+err.Error())
		return
	}

	var sb strings.Builder
	sb.WriteString("# " + d.Name + "\n\n")
	for _, c := range chunks {
		text := modelschunk.ExtractText(c.Content)
		if text == "" {
			continue
		}
		if c.ChunkType == "heading" {
			sb.WriteString("## " + text + "\n\n")
		} else {
			sb.WriteString(text + "\n\n")
		}
	}

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.md"`, d.Name))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(sb.String()))
}

type importMarkdownRequest struct {
	FolderID *string `json:"folder_id,omitempty"`
	Name     string  `json:"name"`
	Markdown string  `json:"markdown"`
}

// ImportMarkdownHandler creates a new rich-text document from raw Markdown,
// splitting on blank lines into paragraph/heading chunks (## -> heading).
func ImportMarkdownHandler(w http.ResponseWriter, r *http.Request) {
	var req importMarkdownRequest
	if err := utils.ReadJSON(r, &req); err != nil || req.Name == "" || req.Markdown == "" {
		utils.WriteError(w, http.StatusBadRequest, "name and markdown are required")
		return
	}

	userID, _ := r.Context().Value("user_id").(string)

	const richTextMimeType = "application/x-zef-doc"
	d, err := dbdocument.CreateDocument(r.Context(), req.FolderID, req.Name, richTextMimeType, 0, "", "", userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to create document: "+err.Error())
		return
	}

	var inputs []modelschunk.ChunkInput
	blocks := strings.Split(req.Markdown, "\n\n")
	idx := 0
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		chunkType := "text"
		text := block
		if strings.HasPrefix(block, "#") {
			chunkType = "heading"
			text = strings.TrimLeft(block, "# ")
		}
		content, _ := json.Marshal(map[string]any{
			"type": "paragraph",
			"content": []map[string]any{
				{"type": "text", "text": text},
			},
		})
		inputs = append(inputs, modelschunk.ChunkInput{ChunkIndex: idx, ChunkType: chunkType, Content: content})
		idx++
	}
	if len(inputs) == 0 {
		inputs = append(inputs, modelschunk.ChunkInput{ChunkIndex: 0, ChunkType: "text", Content: json.RawMessage(`{"type":"paragraph"}`)})
	}

	if _, err := dbchunk.ReplaceChunks(r.Context(), d.DocumentID, inputs); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to save imported content: "+err.Error())
		return
	}

	utils.WriteJSON(w, http.StatusCreated, d)
}
