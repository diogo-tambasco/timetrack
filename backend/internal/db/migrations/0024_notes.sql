-- 0024_notes — fase 26 (plan §2): notas nativas .md (1 linha/nota) + links do slash.
-- workspace_id denormalizado (igual tasks/objectives, D-09 da fase 02) pro filtro
-- tenant trivial (CA-14). Nota é do PROJETO (project_id) ou SOLTA (project_id NULL,
-- governada por created_by — CA-11.1); pode atrelar a uma task (task_id). Markdown
-- puro em body_md. Sem soft-delete: nota é hard delete (CA-06); links caem em cascata.
CREATE TABLE notes (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    project_id   uuid REFERENCES projects(id) ON DELETE CASCADE,   -- NULL = nota solta
    task_id      uuid REFERENCES tasks(id)    ON DELETE SET NULL,  -- atrelada a uma task (opcional)
    title        text NOT NULL DEFAULT '',
    body_md      text NOT NULL DEFAULT '',
    created_by   uuid NOT NULL REFERENCES users(id),
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
-- lista por projeto ordenada por updated_at desc (CA-03/RNF-07)
CREATE INDEX idx_notes_project_updated ON notes (project_id, updated_at DESC);
-- lista de notas soltas do usuário (CA-03.1): só as com project_id NULL
CREATE INDEX idx_notes_loose ON notes (created_by, updated_at DESC) WHERE project_id IS NULL;

CREATE TABLE note_links (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    note_id      uuid NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    ref_type     text NOT NULL CHECK (ref_type IN ('task', 'note')),
    ref_id       uuid NOT NULL,                       -- id da task OU da sub-nota (sem FK: ver D-08)
    position     integer NOT NULL DEFAULT 0,
    workspace_id uuid NOT NULL REFERENCES workspaces(id)   -- denormalizado (CA-14)
);
CREATE INDEX idx_note_links_note ON note_links (note_id, position);
-- evita link duplicado da mesma entidade na mesma nota (defesa-de-banco best-effort — D-08)
CREATE UNIQUE INDEX uq_note_links ON note_links (note_id, ref_type, ref_id);
