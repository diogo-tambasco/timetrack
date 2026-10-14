-- 0021_tasks_v2 — fase 24 (plan §2.1): status ganha 'backlog'; priority, notion_url, position.
-- ⚠️ PG17 PERMITE 'ALTER TYPE … ADD VALUE' DENTRO de uma transação SÓ se o valor novo NÃO for
--    usado no MESMO arquivo. O runner roda CADA .sql na própria tx (BEGIN; Exec(arquivo); COMMIT).
--    Logo: NENHUM statement abaixo pode referenciar 'backlog' (sem DEFAULT/WHERE/CHECK/UPDATE no
--    valor novo). O DEFAULT de status CONTINUA 'todo' (intocado). RNF-05.
ALTER TYPE task_status ADD VALUE 'backlog' BEFORE 'todo';

-- priority 0..3 (0=sem · 1=baixa · 2=média · 3=alta). CHECK é backstop; validação real no Go (D-07).
ALTER TABLE tasks ADD COLUMN priority smallint NOT NULL DEFAULT 0
    CHECK (priority >= 0 AND priority <= 3);

-- notion_url: texto livre opcional (NULL = sem doc). Sem validar formato (preview client-side).
ALTER TABLE tasks ADD COLUMN notion_url text;

-- position: ordenação manual no board/lista (DnD). Default 0; o service grava max+1 no create (D-03).
ALTER TABLE tasks ADD COLUMN position integer NOT NULL DEFAULT 0;

-- índice pro ORDER BY position por coluna (board) — CA-14/17.
CREATE INDEX idx_tasks_project_status_position ON tasks (project_id, status, position);
