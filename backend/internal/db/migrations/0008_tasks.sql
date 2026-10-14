-- 0008_tasks (fase 02): tasks com workspace_id denormalizado (D-09 — filtro
-- tenant direto sem join) e campos multiplayer no schema sem UI no MVP.
CREATE TYPE task_status AS ENUM ('todo', 'doing', 'done');

CREATE TABLE tasks (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    project_id   uuid NOT NULL REFERENCES projects(id),
    name         text NOT NULL,
    status       task_status NOT NULL DEFAULT 'todo',
    due_date     date,
    recurrence   text,                                    -- texto livre / RRULE, opaco nesta fase
    notes        text NOT NULL DEFAULT '',
    created_by   uuid NOT NULL REFERENCES users(id),
    assignee     uuid REFERENCES users(id),               -- base multiplayer, sem UI no MVP
    requested_by uuid REFERENCES users(id),               -- base multiplayer, sem UI no MVP
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_tasks_workspace ON tasks (workspace_id);
CREATE INDEX idx_tasks_project   ON tasks (project_id, status);
