-- 0007_projects (fase 02): projetos com visibilidade privado×compartilhado
-- (CA-09/10) + membros do projeto com acesso edit/view (base multiplayer, CA-11).
CREATE TYPE project_visibility AS ENUM ('private', 'shared');
CREATE TYPE project_access     AS ENUM ('edit', 'view');

CREATE TABLE projects (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id      uuid NOT NULL REFERENCES workspaces(id),
    client_id         uuid REFERENCES clients(id),
    name              text NOT NULL,
    color             text NOT NULL,                      -- '#RRGGBB'
    weekly_hours_goal numeric(5,2),                       -- NULL = sem meta; (0,168]
    visibility        project_visibility NOT NULL,
    owner_user_id     uuid NOT NULL REFERENCES users(id),
    archived_at       timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_projects_workspace ON projects (workspace_id) WHERE archived_at IS NULL;

-- dono do projeto NÃO entra aqui (é implícito — D-06/CA-11)
CREATE TABLE project_members (
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users(id),
    access     project_access NOT NULL,
    PRIMARY KEY (project_id, user_id)
);
