-- 0025_projects_deadline — fase 28 (plan §2): prazo do projeto (card vivo, "tem algo hoje",
-- calendário do projeto). NULL = sem prazo (a maioria não tem — CA-02). + índice de due_date
-- em tasks pro due_today/calendário (RNF-05).
ALTER TABLE projects ADD COLUMN deadline date;   -- NULL default; sem CHECK (date já valida)

-- due_today (summary) e o kind:'task' do calendário filtram por due_date; sem este índice a
-- varredura cresce com as tasks. Parcial (só tasks com due_date — a maioria é NULL).
CREATE INDEX idx_tasks_due ON tasks (project_id, due_date) WHERE due_date IS NOT NULL;
