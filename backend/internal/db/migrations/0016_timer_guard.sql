-- 0016_timer_guard — fase 16 (plan §2): marca de corte automático na entry
-- (anti-timer-esquecido) + subscriptions de Web Push por usuário. Sem cache
-- de eventos novo (RNF-01). Entries antigas viram auto_cut=false sem
-- reprocessamento (o DEFAULT preenche o backstop dos CAs do corte).

ALTER TABLE time_entries
  ADD COLUMN auto_cut boolean NOT NULL DEFAULT false;

-- push_subscriptions: 1 linha por inscrição de navegador. A URL do push
-- service (endpoint) é única por inscrição → PK. ON DELETE CASCADE limpa
-- subs órfãs se o user for removido. Re-inscrever o MESMO navegador
-- substitui as chaves (upsert por endpoint — CA-13).
CREATE TABLE push_subscriptions (
  endpoint    text PRIMARY KEY,
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  p256dh      text NOT NULL,
  auth        text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_push_subs_user ON push_subscriptions (user_id);
