-- 0009_time_entries — fase 03 (plan.md §2): registro de tempo. O servidor é
-- o relógio: duration_seconds é calculado no stop/edição (CA-02). O unique
-- parcial garante NO BANCO o "máx 1 entry rodando por usuário" (CA-03).
-- 'calendar' e 'claude' nascem reservados pras fases 06/08 (D-09).

CREATE TYPE entry_source AS ENUM ('timer', 'manual', 'calendar', 'claude');

CREATE TABLE time_entries (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id     uuid NOT NULL REFERENCES workspaces(id),
  user_id          uuid NOT NULL REFERENCES users(id),
  project_id       uuid REFERENCES projects(id),
  -- task tem hard delete (fase 02 D-03/CA-21): entry preserva o tempo, perde o vínculo
  task_id          uuid REFERENCES tasks(id) ON DELETE SET NULL,
  description      text NOT NULL DEFAULT '',
  tags             text[] NOT NULL DEFAULT '{}',
  started_at       timestamptz NOT NULL,
  stopped_at       timestamptz,
  duration_seconds integer,          -- calculado pelo servidor ao parar/editar
  source           entry_source NOT NULL,
  CONSTRAINT entry_times_valid CHECK (stopped_at IS NULL OR stopped_at > started_at)
);

CREATE INDEX idx_entries_ws_day ON time_entries (workspace_id, started_at DESC);
CREATE INDEX idx_entries_user   ON time_entries (user_id, started_at DESC);
-- CA-03: no máximo 1 entry rodando por usuário, garantido pelo banco
CREATE UNIQUE INDEX uq_entries_one_running
  ON time_entries (user_id) WHERE stopped_at IS NULL;
