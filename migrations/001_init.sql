-- Zef-zoc baseline schema.
-- Consolidates the manually-created base tables (folders, documents,
-- zoc_document_versions — never created via CREATE TABLE anywhere in this
-- repo, reconstructed here from Go query/scan usage) plus everything added
-- by schema_additions.sql (chunk-based rich text, Yjs collab state,
-- comments, trash/archive, tags, views/stars, share links, backlinks,
-- activity, search, document shares, cheque printing). Run manually against
-- Zoc's Postgres — no migration tool in this service, see CLAUDE.md.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================
-- FOLDERS
-- ============================================================
CREATE TABLE IF NOT EXISTS public.folders (
  folder_id   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  parent_id   UUID REFERENCES public.folders(folder_id),
  name        TEXT NOT NULL,
  path        TEXT NOT NULL DEFAULT '',
  created_by  TEXT NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_folders_parent_id ON public.folders(parent_id, created_by);

-- ============================================================
-- DOCUMENTS
-- ============================================================
CREATE TABLE IF NOT EXISTS public.documents (
  document_id  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  folder_id    UUID REFERENCES public.folders(folder_id),
  name         TEXT NOT NULL,
  mime_type    TEXT NOT NULL,
  size_bytes   BIGINT NOT NULL,
  storage_key  TEXT NOT NULL,
  version      INT NOT NULL DEFAULT 1,
  checksum     TEXT,
  created_by   TEXT NOT NULL,
  updated_by   TEXT NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  -- one-for-all document service additions (trash/archive, templates, search)
  archived_at  TIMESTAMPTZ,
  is_template  BOOLEAN NOT NULL DEFAULT false,
  search_text  TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_documents_folder_id ON public.documents(folder_id, created_by);

ALTER TABLE public.documents ADD COLUMN IF NOT EXISTS search_tsv TSVECTOR
  GENERATED ALWAYS AS (setweight(to_tsvector('english', coalesce(name, '')), 'A') ||
                       setweight(to_tsvector('english', coalesce(search_text, '')), 'B')) STORED;
CREATE INDEX IF NOT EXISTS idx_documents_search_tsv ON public.documents USING GIN (search_tsv);

-- ============================================================
-- DOCUMENT VERSIONS
-- ============================================================
CREATE TABLE IF NOT EXISTS public.zoc_document_versions (
  version_id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  document_id      UUID NOT NULL REFERENCES public.documents(document_id) ON DELETE CASCADE,
  version          INT NOT NULL,
  storage_key      TEXT NOT NULL DEFAULT '',
  size_bytes       BIGINT NOT NULL DEFAULT 0,
  checksum         TEXT,
  created_by       TEXT NOT NULL,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  -- version snapshots for chunk-based (rich-text) docs, so restore can
  -- replay content rather than just the blob storage_key/checksum
  chunks_snapshot  JSONB
);

CREATE INDEX IF NOT EXISTS idx_zoc_document_versions_document_id ON public.zoc_document_versions(document_id, version DESC);

-- ============================================================
-- RICH-TEXT DOCUMENT CHUNKS
-- ============================================================
CREATE TABLE IF NOT EXISTS public.zoc_document_chunks (
  chunk_id     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  document_id  UUID NOT NULL REFERENCES public.documents(document_id) ON DELETE CASCADE,
  chunk_index  INT NOT NULL,
  chunk_type   TEXT NOT NULL DEFAULT 'text',
  content      JSONB NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (document_id, chunk_index)
);

CREATE INDEX IF NOT EXISTS idx_zoc_document_chunks_document_id ON public.zoc_document_chunks(document_id);

-- ============================================================
-- YJS COLLAB STATE
-- ============================================================
CREATE TABLE IF NOT EXISTS public.zoc_document_ydoc_state (
  document_id  UUID PRIMARY KEY REFERENCES public.documents(document_id) ON DELETE CASCADE,
  ydoc_state   BYTEA NOT NULL,
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- COMMENTS
-- ============================================================
CREATE TABLE IF NOT EXISTS public.zoc_comments (
  comment_id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  document_id       UUID NOT NULL REFERENCES public.documents(document_id) ON DELETE CASCADE,
  chunk_id          UUID REFERENCES public.zoc_document_chunks(chunk_id) ON DELETE CASCADE,
  parent_comment_id UUID REFERENCES public.zoc_comments(comment_id) ON DELETE CASCADE,
  range_start       INT NOT NULL,
  range_end         INT NOT NULL,
  body              TEXT NOT NULL,
  resolved          BOOLEAN NOT NULL DEFAULT false,
  resolved_by       TEXT,
  resolved_at       TIMESTAMPTZ,
  created_by        TEXT NOT NULL,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_zoc_comments_document_id ON public.zoc_comments(document_id);

-- ============================================================
-- TAGS
-- ============================================================
CREATE TABLE IF NOT EXISTS public.tags (
  tag_id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name         TEXT NOT NULL,
  created_by   TEXT NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (name)
);

CREATE TABLE IF NOT EXISTS public.document_tags (
  document_id  UUID NOT NULL REFERENCES public.documents(document_id) ON DELETE CASCADE,
  tag_id       UUID NOT NULL REFERENCES public.tags(tag_id) ON DELETE CASCADE,
  PRIMARY KEY (document_id, tag_id)
);

-- ============================================================
-- VIEWS / STARS
-- ============================================================
CREATE TABLE IF NOT EXISTS public.document_views (
  document_id  UUID NOT NULL REFERENCES public.documents(document_id) ON DELETE CASCADE,
  user_id      TEXT NOT NULL,
  viewed_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (document_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_document_views_user_viewed ON public.document_views(user_id, viewed_at DESC);

CREATE TABLE IF NOT EXISTS public.document_stars (
  document_id  UUID NOT NULL REFERENCES public.documents(document_id) ON DELETE CASCADE,
  user_id      TEXT NOT NULL,
  starred_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (document_id, user_id)
);

-- ============================================================
-- SHARE LINKS / DOCUMENT SHARES
-- ============================================================
CREATE TABLE IF NOT EXISTS public.share_links (
  share_id     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  token        TEXT NOT NULL UNIQUE,
  document_id  UUID NOT NULL REFERENCES public.documents(document_id) ON DELETE CASCADE,
  permission   TEXT NOT NULL DEFAULT 'view', -- view | comment
  created_by   TEXT NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at   TIMESTAMPTZ,
  revoked_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_share_links_document_id ON public.share_links(document_id);

CREATE TABLE IF NOT EXISTS public.document_shares (
  document_id  UUID NOT NULL REFERENCES public.documents(document_id) ON DELETE CASCADE,
  user_id      TEXT NOT NULL,
  permission   TEXT NOT NULL DEFAULT 'view', -- view | comment | edit
  created_by   TEXT NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (document_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_document_shares_user_id ON public.document_shares(user_id);

-- ============================================================
-- BACKLINKS
-- ============================================================
CREATE TABLE IF NOT EXISTS public.document_links (
  source_document_id UUID NOT NULL REFERENCES public.documents(document_id) ON DELETE CASCADE,
  target_document_id UUID NOT NULL REFERENCES public.documents(document_id) ON DELETE CASCADE,
  source_chunk_id     UUID REFERENCES public.zoc_document_chunks(chunk_id) ON DELETE CASCADE,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (source_document_id, target_document_id, source_chunk_id)
);

CREATE INDEX IF NOT EXISTS idx_document_links_target ON public.document_links(target_document_id);

-- ============================================================
-- ACTIVITY
-- ============================================================
CREATE TABLE IF NOT EXISTS public.document_activity (
  activity_id  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  document_id  UUID NOT NULL REFERENCES public.documents(document_id) ON DELETE CASCADE,
  user_id      TEXT NOT NULL,
  action       TEXT NOT NULL, -- created | edited | commented | shared | restored | deleted | tagged | mentioned
  metadata     JSONB,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_document_activity_document_id ON public.document_activity(document_id, created_at DESC);
