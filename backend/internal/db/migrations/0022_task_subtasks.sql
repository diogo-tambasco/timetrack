-- 0022_task_subtasks — fase 24 (plan §2.2): sub-tasks "goals da task". workspace_id denormalizado
-- (D-09 herdado) pro filtro tenant trivial. NÃO referencia objectives (separado dos Goals do projeto).
CREATE TABLE task_subtasks (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id      uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    title        text NOT NULL,
    done         boolean NOT NULL DEFAULT false,
    is_goal      boolean NOT NULL DEFAULT false,
    position     integer NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_subtasks_task ON task_subtasks (task_id, position);
-- agregado de progresso/metas do board (CA-14/RNF-04): contagem por task com filtro is_goal.
CREATE INDEX idx_subtasks_task_goal ON task_subtasks (task_id) WHERE is_goal;
