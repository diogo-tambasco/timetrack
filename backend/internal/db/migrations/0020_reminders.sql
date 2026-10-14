-- 0020_reminders — fase 20 (plan §2): lembretes (due/manual). user_id = quem é
-- lembrado. Worker próprio (internal/reminders) varre a fatia pending pelo
-- idx_reminders_pending; o uq_reminders_due_pending garante no máximo 1 reminder
-- due pendente por task (backstop da idempotência §2.2 — CA-03/12). FKs ON
-- DELETE CASCADE limpam reminders órfãos quando a task/o user some.

CREATE TABLE reminders (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  task_id      uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  workspace_id uuid NOT NULL REFERENCES workspaces(id),              -- denorm (filtro tenant, igual tasks)
  remind_at    timestamptz NOT NULL,
  kind         text NOT NULL CHECK (kind IN ('due','manual')),
  status       text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sent','canceled')),
  created_at   timestamptz NOT NULL DEFAULT now()
);

-- varredura barata do worker (RNF-01): só a fatia pending, ordenável por remind_at.
CREATE INDEX idx_reminders_pending ON reminders (remind_at) WHERE status = 'pending';

-- no máximo 1 reminder due pendente por task (backstop da idempotência §2.2 — CA-03/12).
CREATE UNIQUE INDEX uq_reminders_due_pending ON reminders (task_id)
  WHERE kind = 'due' AND status = 'pending';

-- listar os meus (CA-20) — por user_id; remind_at pra ordenar.
CREATE INDEX idx_reminders_user ON reminders (user_id, remind_at);
