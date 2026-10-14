-- 0018_task_assignment_idx — fase 18 (plan §2): índices p/ as visões cross-workspace
-- ("Minhas tasks" / "Pedidas por mim"). As colunas tasks.assignee / tasks.requested_by
-- já existem (0008_tasks, uuid nullable FK a users). NENHUMA coluna nova.
-- Índices PARCIAIS: a vasta maioria das tasks tem os campos nulos; o índice só
-- precisa cobrir as atribuídas/pedidas — menor e mais barato pra GET /tasks/assigned.
CREATE INDEX idx_tasks_assignee     ON tasks (assignee)     WHERE assignee IS NOT NULL;
CREATE INDEX idx_tasks_requested_by ON tasks (requested_by) WHERE requested_by IS NOT NULL;
