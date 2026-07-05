package routes

import (
	"net/http"

	activityHandlers "zoc/src/internal/handlers/activity"
	aiHandlers "zoc/src/internal/handlers/ai"
	chunkHandlers "zoc/src/internal/handlers/chunk"
	commentHandlers "zoc/src/internal/handlers/comment"
	documentHandlers "zoc/src/internal/handlers/document"
	exportHandlers "zoc/src/internal/handlers/export"
	folderHandlers "zoc/src/internal/handlers/folder"
	imageHandlers "zoc/src/internal/handlers/image"
	searchHandlers "zoc/src/internal/handlers/search"
	shareHandlers "zoc/src/internal/handlers/share"
	tagHandlers "zoc/src/internal/handlers/tag"
	"zoc/src/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()

	r.Use(utils.CORSMiddleware)
	r.Use(conditionalLogger)
	r.Use(handlerLogger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	r.Route("/folders", func(r chi.Router) {
		r.Use(JWTMiddleware)
		r.Post("/", folderHandlers.CreateFolderHandler)
		r.Get("/", folderHandlers.ListFoldersHandler)
		r.Get("/trash", folderHandlers.ListTrashedFoldersHandler)
		r.Get("/{id}", folderHandlers.GetFolderHandler)
		r.Patch("/{id}", folderHandlers.UpdateFolderHandler)
		r.Post("/{id}/move", folderHandlers.MoveFolderHandler)
		r.Delete("/{id}", folderHandlers.DeleteFolderHandler)
		r.Post("/{id}/restore", folderHandlers.RestoreFolderHandler)
	})

	r.Route("/documents", func(r chi.Router) {
		r.Use(JWTMiddleware)
		r.Post("/", documentHandlers.CreateDocumentHandler)
		r.Get("/", documentHandlers.ListDocumentsHandler)
		r.Get("/trash", documentHandlers.ListTrashedDocumentsHandler)
		r.Get("/recent", tagHandlers.RecentDocumentsHandler)
		r.Get("/starred", tagHandlers.ListStarredDocumentsHandler)
		r.Get("/templates", documentHandlers.ListTemplatesHandler)
		r.Post("/bulk-move", documentHandlers.BulkMoveDocumentsHandler)
		r.Post("/import/markdown", exportHandlers.ImportMarkdownHandler)

		r.Get("/{id}", documentHandlers.GetDocumentHandler)
		r.Patch("/{id}", documentHandlers.UpdateDocumentHandler)
		r.Delete("/{id}", documentHandlers.DeleteDocumentHandler)
		r.Post("/{id}/restore", documentHandlers.RestoreDocumentHandler)
		r.Post("/{id}/archive", documentHandlers.ArchiveDocumentHandler)
		r.Post("/{id}/unarchive", documentHandlers.UnarchiveDocumentHandler)
		r.Post("/{id}/duplicate", documentHandlers.DuplicateDocumentHandler)

		r.Get("/{id}/versions", documentHandlers.ListDocumentVersionsHandler)
		r.Post("/{id}/versions/{version}/restore", documentHandlers.RestoreVersionHandler)
		r.Get("/{id}/versions/{v1}/diff/{v2}", documentHandlers.DiffVersionsHandler)

		r.Get("/{id}/content", chunkHandlers.GetDocumentContentHandler)
		r.Put("/{id}/content", chunkHandlers.SaveDocumentContentHandler)
		r.Post("/{id}/images", imageHandlers.UploadImageHandler)

		r.Post("/{id}/comments", commentHandlers.CreateCommentHandler)
		r.Get("/{id}/comments", commentHandlers.ListCommentsHandler)
		r.Post("/{id}/comments/{commentId}/resolve", commentHandlers.ResolveCommentHandler)
		r.Post("/{id}/comments/{commentId}/unresolve", commentHandlers.UnresolveCommentHandler)
		r.Delete("/{id}/comments/{commentId}", commentHandlers.DeleteCommentHandler)

		r.Get("/{id}/tags", tagHandlers.ListDocumentTagsHandler)
		r.Post("/{id}/tags/{tagId}", tagHandlers.AddDocumentTagHandler)
		r.Delete("/{id}/tags/{tagId}", tagHandlers.RemoveDocumentTagHandler)
		r.Post("/{id}/star", tagHandlers.StarDocumentHandler)
		r.Delete("/{id}/star", tagHandlers.UnstarDocumentHandler)

		r.Post("/{id}/template", documentHandlers.MarkTemplateHandler)
		r.Delete("/{id}/template", documentHandlers.UnmarkTemplateHandler)

		r.Get("/{id}/backlinks", documentHandlers.BacklinksHandler)
		r.Get("/{id}/toc", documentHandlers.TOCHandler)

		r.Get("/{id}/activity", activityHandlers.ListActivityHandler)

		r.Get("/{id}/export", exportHandlers.ExportMarkdownHandler)

		r.Post("/{id}/share", shareHandlers.CreateShareLinkHandler)
		r.Get("/{id}/share", shareHandlers.ListShareLinksHandler)
		r.Delete("/{id}/share/{token}", shareHandlers.RevokeShareLinkHandler)

		r.Post("/{id}/summarize", aiHandlers.SummarizeDocumentHandler)
		r.Post("/{id}/ask", aiHandlers.AskDocumentHandler)
	})

	r.Route("/tags", func(r chi.Router) {
		r.Use(JWTMiddleware)
		r.Post("/", tagHandlers.CreateTagHandler)
		r.Get("/", tagHandlers.ListTagsHandler)
		r.Delete("/{tagId}", tagHandlers.DeleteTagHandler)
	})

	r.Route("/search", func(r chi.Router) {
		r.Use(JWTMiddleware)
		r.Get("/", searchHandlers.SearchHandler)
	})

	// Unauthenticated: read-only access via a share token, not a user JWT.
	r.Route("/shared/{token}", func(r chi.Router) {
		r.Get("/", shareHandlers.GetSharedContentHandler)
	})

	r.Route("/internal/documents", func(r chi.Router) {
		r.Use(InternalServiceMiddleware)
		r.Get("/{id}/ydoc", chunkHandlers.InternalGetYDocHandler)
		r.Put("/{id}/ydoc", chunkHandlers.InternalSaveYDocHandler)
		r.Get("/{id}/content", chunkHandlers.GetDocumentContentHandler)
	})

	return r
}
