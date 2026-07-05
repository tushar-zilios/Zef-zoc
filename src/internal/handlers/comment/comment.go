package handlers

import (
	"errors"
	"net/http"
	"regexp"

	dbactivity "zoc/src/internal/db/activity"
	dbcomment "zoc/src/internal/db/comment"
	models "zoc/src/internal/models/comment"
	"zoc/src/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

var mentionPattern = regexp.MustCompile(`@([a-zA-Z0-9_-]+)`)

type createCommentRequest struct {
	ChunkID         *string `json:"chunk_id,omitempty"`
	ParentCommentID *string `json:"parent_comment_id,omitempty"`
	RangeStart      int     `json:"range_start"`
	RangeEnd        int     `json:"range_end"`
	Body            string  `json:"body"`
}

func CreateCommentHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	var req createCommentRequest
	if err := utils.ReadJSON(r, &req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Body == "" {
		utils.WriteError(w, http.StatusBadRequest, "body is required")
		return
	}

	userID, _ := r.Context().Value("user_id").(string)

	c, err := dbcomment.CreateComment(r.Context(), documentID, req.ChunkID, req.ParentCommentID, req.RangeStart, req.RangeEnd, req.Body, userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to create comment: "+err.Error())
		return
	}

	_ = dbactivity.LogActivity(r.Context(), documentID, userID, "commented", map[string]string{"comment_id": c.CommentID})
	for _, match := range mentionPattern.FindAllStringSubmatch(req.Body, -1) {
		_ = dbactivity.LogActivity(r.Context(), documentID, userID, "mentioned", map[string]string{"mentioned_user": match[1], "comment_id": c.CommentID})
	}

	utils.WriteJSON(w, http.StatusCreated, c)
}

func ListCommentsHandler(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "id")
	list, err := dbcomment.ListComments(r.Context(), documentID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to list comments: "+err.Error())
		return
	}
	if list == nil {
		list = []models.Comment{}
	}
	utils.WriteJSON(w, http.StatusOK, list)
}

func ResolveCommentHandler(w http.ResponseWriter, r *http.Request) {
	setResolved(w, r, true)
}

func UnresolveCommentHandler(w http.ResponseWriter, r *http.Request) {
	setResolved(w, r, false)
}

func setResolved(w http.ResponseWriter, r *http.Request, resolved bool) {
	commentID := chi.URLParam(r, "commentId")
	userID, _ := r.Context().Value("user_id").(string)

	c, err := dbcomment.ResolveComment(r.Context(), commentID, resolved, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.WriteError(w, http.StatusNotFound, "Comment not found")
			return
		}
		utils.WriteError(w, http.StatusInternalServerError, "Failed to update comment: "+err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusOK, c)
}

func DeleteCommentHandler(w http.ResponseWriter, r *http.Request) {
	commentID := chi.URLParam(r, "commentId")
	if err := dbcomment.DeleteComment(r.Context(), commentID); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to delete comment: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
