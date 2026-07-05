# Zef-zoc

## DATABASE OWNERSHIP
This service owns **one specific Supabase PostgreSQL database**:
- Env var: `DATABASE_URL` in `Zef-zoc/.env`
- Tables (`folders`, `documents`, `zoc_document_versions`, `zoc_document_chunks`, `zoc_document_ydoc_state`, `zoc_comments`) are created manually — this service does NOT run `CREATE TABLE` migrations on boot, unlike `Zef-cto`/`Zef-backend`. See `schema_additions.sql` for the chunk/ydoc/comments tables.

**DO NOT** read or modify DB schemas/queries from `Zef-backend/`, `Zef-cto/`, or `Zef-accountant/`. Those are separate services with separate databases.

## This service handles
- Zoc: folder/document metadata storage (nested folders, document records, version history)
- Rich-text documents (`mime_type = 'application/x-zef-doc'`): content stored as chunks in `zoc_document_chunks`, bypassing GCS blob storage. See `/documents/{id}/content` (GET/PUT) and `/internal/documents/{id}/ydoc` (service-secret protected, called by `Zef-zoc-collab`).
- Image uploads for the editor: `POST /documents/{id}/images`, reuses the GCS client, returns a public HTTPS URL.
- Threaded comments anchored to a chunk + text range: `/documents/{id}/comments`.
- Port 8086

Real-time collaborative editing (Yjs CRDT sync) is handled by the separate `Zef-zoc-collab` Node/Hocuspocus sidecar (port 8090), which persists back to this service via the `/internal/documents/{id}/ydoc` routes rather than touching Postgres directly.

Env additions for the above: `INTERNAL_SERVICE_SECRET` (shared secret `Zef-zoc-collab` sends as `X-Internal-Service-Key`).

Run: `cd Zef-zoc && go run src/cmd/server/main.go` (or via root `make run-zoc`)
