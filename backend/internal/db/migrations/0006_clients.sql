-- 0006_clients (fase 02): clientes do workspace, soft-delete via archived_at (CA-05).
CREATE TABLE clients (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    name         text NOT NULL,
    archived_at  timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_clients_workspace ON clients (workspace_id) WHERE archived_at IS NULL;
