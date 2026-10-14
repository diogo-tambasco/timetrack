-- summary_seed.sql — fase 28 CA-07 (perf): 30 projetos, 300 tasks, 1.000
-- entries num workspace. Placeholders {{WORKSPACE_ID}}/{{USER_ID}} substituídos
-- pelo teste (padrão seed500.sql/board_seed.sql). Mede que o /projects/summary
-- responde < 200ms (sem N+1 — RNF-04/05).

-- 30 projetos shared do user
INSERT INTO projects (workspace_id, owner_user_id, name, color, visibility)
SELECT '{{WORKSPACE_ID}}', '{{USER_ID}}', 'Proj ' || g, '#4f9dde', 'shared'
FROM generate_series(1, 30) g;

-- 300 tasks distribuídas round-robin pelos projetos (status e due_date variados)
INSERT INTO tasks (workspace_id, project_id, name, created_by, status, due_date)
SELECT '{{WORKSPACE_ID}}',
       (SELECT id FROM projects
         WHERE workspace_id = '{{WORKSPACE_ID}}'
         ORDER BY created_at, id OFFSET (g % 30) LIMIT 1),
       'Task ' || g, '{{USER_ID}}',
       (ARRAY['todo','doing','done'])[1 + (g % 3)]::task_status,
       CASE WHEN g % 5 = 0 THEN current_date ELSE NULL END
FROM generate_series(1, 300) g;

-- 1.000 entries fechadas (10min cada) distribuídas round-robin pelos projetos
INSERT INTO time_entries (workspace_id, user_id, project_id, started_at, stopped_at, duration_seconds, source)
SELECT '{{WORKSPACE_ID}}', '{{USER_ID}}',
       (SELECT id FROM projects
         WHERE workspace_id = '{{WORKSPACE_ID}}'
         ORDER BY created_at, id OFFSET (g % 30) LIMIT 1),
       now() - make_interval(mins => g + 10), now() - make_interval(mins => g), 600, 'manual'
FROM generate_series(1, 1000) g;
