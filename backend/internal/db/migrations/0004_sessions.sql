-- 0004_sessions (fase 01): sessão opaca — só o hash do token no banco (CA-08).
CREATE TABLE sessions (
  token_hash text PRIMARY KEY,                  -- sha256 hex do token do cookie
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL
);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);
