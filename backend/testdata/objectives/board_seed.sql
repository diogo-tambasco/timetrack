-- board_seed.sql — fase 12 CA-18 (perf): 30 objetivos, 300 tasks, 1.000 entries
-- num projeto. Placeholders {{WORKSPACE_ID}}/{{USER_ID}}/{{PROJECT_ID}} são
-- substituídos pelo teste (padrão seed500.sql da fase 02). Mede que o /board
-- responde < 200ms (sem N+1 — RNF-04).

-- 30 objetivos
INSERT INTO objectives (workspace_id, project_id, name, position, created_by)
SELECT '{{WORKSPACE_ID}}', '{{PROJECT_ID}}', 'Obj ' || g, g, '{{USER_ID}}'
FROM generate_series(1, 30) g;

-- 300 tasks distribuídas round-robin pelos objetivos (status variado)
INSERT INTO tasks (workspace_id, project_id, name, created_by, objective_id, status)
SELECT '{{WORKSPACE_ID}}', '{{PROJECT_ID}}', 'Task ' || g, '{{USER_ID}}',
       (SELECT id FROM objectives
         WHERE workspace_id = '{{WORKSPACE_ID}}' AND project_id = '{{PROJECT_ID}}'
         ORDER BY position OFFSET (g % 30) LIMIT 1),
       (ARRAY['todo','doing','done'])[1 + (g % 3)]::task_status
FROM generate_series(1, 300) g;

-- 1.000 entries fechadas (10min cada) distribuídas round-robin pelas tasks
INSERT INTO time_entries (workspace_id, user_id, project_id, task_id, started_at, stopped_at, duration_seconds, source)
SELECT '{{WORKSPACE_ID}}', '{{USER_ID}}', '{{PROJECT_ID}}',
       (SELECT id FROM tasks
         WHERE workspace_id = '{{WORKSPACE_ID}}' AND project_id = '{{PROJECT_ID}}'
         ORDER BY created_at, id OFFSET (g % 300) LIMIT 1),
       now() - make_interval(mins => g + 10), now() - make_interval(mins => g), 600, 'manual'
FROM generate_series(1, 1000) g;
