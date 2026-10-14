-- 0014_report_settings — fase 14: preferências do relatório POR USUÁRIO
-- (jornada útil esperada — base do rastreado×não-rastreado, CA-21). Espelha
-- calendar_settings (fase 11). Sem linha = defaults na leitura.
CREATE TABLE report_settings (
  user_id            uuid PRIMARY KEY REFERENCES users(id),
  work_hours_per_day numeric(4,2) NOT NULL DEFAULT 8   CHECK (work_hours_per_day > 0 AND work_hours_per_day <= 24),
  workdays_only      boolean      NOT NULL DEFAULT true,
  updated_at         timestamptz  NOT NULL DEFAULT now()
);
