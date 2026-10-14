-- 0026_objectives_modes — fase 29 (plan §2): Goals com modos de conclusão.
-- by_tasks = comportamento da fase 12 (% tasks done, auto-close a 100%) — DEFAULT,
-- preserva todos os objetivos existentes. by_hours = tracked_seconds vs goal_hours.
-- by_date = target_date é marco/milestone (fecho manual; alimenta o calendário, fase 28).
-- Migração ADITIVA: coluna com DEFAULT + coluna nullable. ALTER TABLE ADD COLUMN roda
-- na própria transação do runner (não é ALTER TYPE ADD VALUE — sem a restrição do §9.3
-- do PRD; não há enum novo nesta fase: completion_mode é text + CHECK).
ALTER TABLE objectives
  ADD COLUMN completion_mode text NOT NULL DEFAULT 'by_tasks'
    CHECK (completion_mode IN ('by_tasks','by_hours','by_date')),
  ADD COLUMN target_date date;
