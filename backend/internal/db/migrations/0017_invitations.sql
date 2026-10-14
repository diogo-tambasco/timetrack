-- 0017_invitations — fase 17 (plan §2): convites de workspace (multiplayer).
-- 1ª tabela que materializa entrada de gente por API (até aqui só o 1º login
-- criava o owner). Reusa o enum workspace_role (0003). O service grava sempre
-- email normalizado (lower/trim) e rejeita role='owner' no payload (CA-03).
-- O token opaco fica pronto pra um futuro "link de convite" — nunca vai pro
-- corpo de resposta (RNF-04). O aceite desta fase é por LOGIN do email.
CREATE TABLE invitations (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  email       text NOT NULL,                          -- normalizado lower/trim pelo service
  role        workspace_role NOT NULL CHECK (role <> 'owner'),  -- owner só por PATCH (CA-03)
  token       text NOT NULL UNIQUE,                   -- opaco, server-side (RNF-03/04)
  status      text NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending', 'accepted', 'revoked')),
  invited_by  uuid NOT NULL REFERENCES users(id),
  created_at  timestamptz NOT NULL DEFAULT now(),
  expires_at  timestamptz                              -- NULL = sem expiração; service preenche now()+TTL
);

-- índice único PARCIAL (CA-02): no máximo 1 pending por (workspace, email),
-- case-insensitive. Convidar o mesmo email 2× sem revogar/aceitar bate aqui
-- (o service trata como invite_pending, não 500). Revogar/aceitar libera um
-- novo pending (CA-09 — o índice só barra pending↔pending).
CREATE UNIQUE INDEX ux_invitations_pending
  ON invitations (workspace_id, lower(email)) WHERE status = 'pending';

-- índice de resolução no login (RNF-02): a varredura do 1º login busca por
-- email normalizado entre os pendentes.
CREATE INDEX idx_invitations_email_pending
  ON invitations (lower(email)) WHERE status = 'pending';
