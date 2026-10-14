-- 0023_task_properties — fase 25 (plan §2): "database por projeto".
-- project_property_defs = o SCHEMA de propriedades custom de um projeto.
-- task_property_values  = o VALOR de uma def numa task (jsonb tipado).
-- workspace_id denormalizado (D-09 da fase 02) pro filtro tenant trivial (CA-15).
-- O CHECK de type lista os 8 tipos (schema comporta person/multi_select), mas o
-- SERVICE da v1 restringe aos 6 (text/number/select/date/url/checkbox) — CA-02/§6.

CREATE TABLE project_property_defs (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    name         text NOT NULL,
    type         text NOT NULL CHECK (type IN
                   ('text','number','select','multi_select','date','person','checkbox','url')),
    options      jsonb NOT NULL DEFAULT '[]'::jsonb,   -- select/multi_select: [{id,label,color}]
    position     integer NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_prop_defs_project ON project_property_defs (project_id, position);

CREATE TABLE task_property_values (
    task_id      uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    def_id       uuid NOT NULL REFERENCES project_property_defs(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    value        jsonb NOT NULL,                        -- tipado conforme def.type (CA-09)
    PRIMARY KEY (task_id, def_id)
);
-- leitura do /board (CA-16/RNF-02): values do projeto = WHERE def_id = ANY($defs).
CREATE INDEX idx_prop_values_def ON task_property_values (def_id);
