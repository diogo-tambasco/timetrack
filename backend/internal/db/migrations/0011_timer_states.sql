-- 0011_timer_states — fase 03 (plan.md §2, D-03): estado de RUNTIME do
-- timer, 1 linha por usuário (PK reforça "1 timer por usuário"). Detalhe
-- INTERNO da fase: o contrato público é o TimerState do GET /timer
-- (specs/INDEX.md) — a fase 04 consome timer.Service/OnChange, nunca lê
-- esta tabela. Ciclo: start cria a linha · stop remove · pause vira
-- mode='break' com resume_* preenchidos · resume/skip volta pro trabalho.

CREATE TYPE timer_mode AS ENUM ('countup', 'countdown', 'pomodoro_focus', 'break');

CREATE TABLE timer_states (
  user_id                uuid PRIMARY KEY REFERENCES users(id),
  mode                   timer_mode NOT NULL,
  -- ON DELETE SET NULL = defesa em profundidade; a API bloqueia antes com 409 (CA-24.1/24.2)
  entry_id               uuid REFERENCES time_entries(id) ON DELETE SET NULL,  -- NULL quando mode='break'
  target_seconds         integer,            -- alvo (countdown/pomodoro_focus/break)
  started_at             timestamptz NOT NULL,
  -- contexto pra retomar depois da pausa:
  resume_entry_id        uuid REFERENCES time_entries(id) ON DELETE SET NULL,  -- entry a clonar no resume
  resume_mode            timer_mode,                        -- modo a restaurar
  resume_target_seconds  integer                            -- restante do alvo (CA-10)
);
