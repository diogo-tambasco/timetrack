-- 0010_pomodoro_settings — fase 03 (plan.md §2): durações do Pomodoro POR
-- USUÁRIO (D-08 — a pausa é hábito da pessoa, vale em qualquer workspace).
-- Sem linha = defaults 25/5 aplicados na leitura (CA-19).

CREATE TABLE pomodoro_settings (
  user_id       uuid PRIMARY KEY REFERENCES users(id),
  focus_minutes integer NOT NULL DEFAULT 25 CHECK (focus_minutes BETWEEN 15 AND 90),
  break_minutes integer NOT NULL DEFAULT 5  CHECK (break_minutes BETWEEN 1 AND 60),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
