-- board_props_seed.sql — fase 25 CA-18 (perf): 10 defs, 300 tasks, ~3.000 values
-- (cada task × cada def) num projeto. Placeholders {{WORKSPACE_ID}}/{{USER_ID}}/
-- {{PROJECT_ID}} substituídos pelo teste (padrão board_seed.sql da fase 12). Mede que
-- o /board responde < 200ms (sem N+1: defs em 1 query, values em 1 query — RNF-02).

-- 10 defs do tipo text (position 0..9)
INSERT INTO project_property_defs (project_id, workspace_id, name, type, position)
SELECT '{{PROJECT_ID}}', '{{WORKSPACE_ID}}', 'Def ' || g, 'text', g - 1
FROM generate_series(1, 10) g;

-- 300 tasks
INSERT INTO tasks (workspace_id, project_id, name, created_by)
SELECT '{{WORKSPACE_ID}}', '{{PROJECT_ID}}', 'Task ' || g, '{{USER_ID}}'
FROM generate_series(1, 300) g;

-- ~3.000 values: cada task × cada def
INSERT INTO task_property_values (task_id, def_id, workspace_id, value)
SELECT t.id, d.id, '{{WORKSPACE_ID}}', '"v"'::jsonb
  FROM (SELECT id FROM tasks
         WHERE workspace_id = '{{WORKSPACE_ID}}' AND project_id = '{{PROJECT_ID}}') t
 CROSS JOIN (SELECT id FROM project_property_defs
              WHERE project_id = '{{PROJECT_ID}}') d;
