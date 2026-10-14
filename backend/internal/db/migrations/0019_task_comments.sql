-- 0019_task_comments — fase 19 (plan §2): thread de comentários por task.
-- workspace_id denormalizado (igual tasks, D-09 da fase 02) → filtro tenant
-- direto. ON DELETE CASCADE no task_id: a thread morre com a task (CA-01.1).

CREATE TABLE task_comments (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id      uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  author_id    uuid NOT NULL REFERENCES users(id),
  body         text NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_task_comments_task ON task_comments (task_id, created_at);
