-- Seed de carga da fase 02 (CA-32, T-12): 500 registros na proporção fixa
-- 50 clientes / 150 projetos / 300 tasks num único workspace.
-- Os placeholders {{WORKSPACE_ID}} e {{USER_ID}} são substituídos pelo
-- helper do teste antes da execução (seed reutilizável pelas fases 03+).

INSERT INTO clients (workspace_id, name)
SELECT '{{WORKSPACE_ID}}', 'Cliente ' || i
  FROM generate_series(1, 50) AS i;

WITH cs AS (
  SELECT array_agg(id) AS ids
    FROM clients
   WHERE workspace_id = '{{WORKSPACE_ID}}'
)
INSERT INTO projects (workspace_id, client_id, name, color, weekly_hours_goal, visibility, owner_user_id)
SELECT '{{WORKSPACE_ID}}',
       cs.ids[(i % 50) + 1],
       'Projeto ' || i,
       '#4f9dde',
       CASE WHEN i % 3 = 0 THEN NULL ELSE 10 END,
       'shared',
       '{{USER_ID}}'
  FROM generate_series(1, 150) AS i, cs;

WITH ps AS (
  SELECT array_agg(id) AS ids
    FROM projects
   WHERE workspace_id = '{{WORKSPACE_ID}}'
)
INSERT INTO tasks (workspace_id, project_id, name, status, created_by)
SELECT '{{WORKSPACE_ID}}',
       ps.ids[((i - 1) % 150) + 1],
       'Task ' || i,
       (ARRAY['todo','doing','done']::task_status[])[(i % 3) + 1],
       '{{USER_ID}}'
  FROM generate_series(1, 300) AS i, ps;
