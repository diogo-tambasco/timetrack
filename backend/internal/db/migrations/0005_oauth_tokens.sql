-- 0005_oauth_tokens (fase 01): tokens Google cifrados em repouso (CA-14).
-- Definição CANÔNICA — a fase 06 referencia, não redefine.
CREATE TABLE oauth_tokens (
  user_id           uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider          text NOT NULL DEFAULT 'google',
  access_token_enc  bytea NOT NULL,             -- AES-256-GCM (nonce || ciphertext)
  refresh_token_enc bytea,
  token_expires_at  timestamptz,
  scopes            text[] NOT NULL,
  updated_at        timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, provider)               -- permite outro provider no futuro
);
