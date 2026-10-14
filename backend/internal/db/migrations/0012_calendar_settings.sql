-- 0012_calendar_settings — fase 11 (plan §2): preferências da UI de
-- calendário POR USUÁRIO (sincroniza entre dispositivos). Sem linha =
-- defaults aplicados na leitura (CA-01).
CREATE TABLE calendar_settings (
  user_id      uuid PRIMARY KEY REFERENCES users(id),
  default_view text NOT NULL DEFAULT 'week'        CHECK (default_view IN ('day','week','month')),
  hour_start   integer NOT NULL DEFAULT 6          CHECK (hour_start BETWEEN 0 AND 23),
  hour_end     integer NOT NULL DEFAULT 23         CHECK (hour_end BETWEEN 1 AND 24),
  density      text NOT NULL DEFAULT 'comfortable' CHECK (density IN ('comfortable','compact')),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  CHECK (hour_end > hour_start)
);
