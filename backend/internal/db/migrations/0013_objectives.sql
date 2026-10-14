-- 0013_objectives — fase 12 (plan §2): camada objetivos entre projeto e task.
-- workspace_id denormalizado (igual tasks, D-09 da fase 02) pro filtro tenant
-- trivial (CA-15). Sem soft-delete: objetivo é open/done + hard delete; tasks
-- que o referenciam caem pra NULL (bucket "Geral", CA-07/11).
-- goal_hours numeric(6,2): cabe a faixa declarada (0,9999] — numeric(5,2) só ia a
-- 999.99 e estouraria (22003/500) em 9999 (furo confirmado na verificação da spec).
CREATE TYPE objective_status AS ENUM ('open', 'done');

CREATE TABLE objectives (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    project_id   uuid NOT NULL REFERENCES projects(id),
    name         text NOT NULL,
    description  text NOT NULL DEFAULT '',
    status       objective_status NOT NULL DEFAULT 'open',
    goal_hours   numeric(6,2) CHECK (goal_hours IS NULL OR (goal_hours > 0 AND goal_hours <= 9999)),
    position     integer NOT NULL DEFAULT 0,
    created_by   uuid NOT NULL REFERENCES users(id),
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_objectives_project ON objectives (project_id, position);

-- ON DELETE SET NULL: única forma de remover objetivo (CA-07) sem apagar tasks;
-- espelha time_entries.task_id ON DELETE SET NULL da fase 03.
ALTER TABLE tasks
    ADD COLUMN objective_id uuid REFERENCES objectives(id) ON DELETE SET NULL;
CREATE INDEX idx_tasks_objective ON tasks (objective_id);

-- soma de tempo por task do /board (CA-16/RNF-04): os índices da 0009 cobrem só
-- user_id/workspace_id+started_at; este cobre o GROUP BY task_id das entries fechadas.
CREATE INDEX idx_entries_task ON time_entries (task_id) WHERE duration_seconds IS NOT NULL;
