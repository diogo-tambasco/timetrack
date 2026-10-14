-- 0002_users (fase 01). pgcrypto já habilitado pela 0001_init.sql da fase 00.
CREATE TABLE users (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  google_id   text NOT NULL UNIQUE,
  email       text NOT NULL UNIQUE,
  name        text NOT NULL,
  avatar_url  text,
  created_at  timestamptz NOT NULL DEFAULT now()
);
