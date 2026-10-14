// assigned.go — fase 18 T-06: visões cross-workspace "Minhas tasks" (Assigned)
// e "Pedidas por mim" (Requested). Rotas POR USUÁRIO (sem workspace na URL —
// cruzam todos os workspaces que o user participa). Cada item enriquece a Task
// com o contexto cross-ws: projeto (id/name/color), workspace (id/name) e
// cliente (name|null).
//
// Isolamento (RNF-01/CA-21): 3 filtros load-bearing, cada um provado por mutação
// no assigned_test.go —
//  1. WHERE t.assignee=$1 (ou requested_by) — só as MINHAS;
//  2. JOIN workspace_members wm ON ... wm.user_id=$1 — só ws que EU participo
//     (uma task num ws que deixei/nunca participei NÃO vaza);
//  3. AND (p.visibility='shared' OR p.owner_user_id=$1) — visibilidade do projeto
//     (private de outro não aparece; private meu aparece — CA-20).
package tasks

import "context"

// AssignedTask é uma Task com o contexto cross-workspace pra as visões.
type AssignedTask struct {
	Task
	Project struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Color string `json:"color"`
	} `json:"project"`
	Workspace struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"workspace"`
	Client *string `json:"client"` // nome do cliente ou null
}

// assignedView roda a query enriquecida filtrando pela coluna dada
// (t.assignee ou t.requested_by) = userID. Uma só query (sem N+1 — RNF-04).
func (s *Service) assignedView(ctx context.Context, col, userID string) ([]AssignedTask, error) {
	// col é um identificador FIXO controlado internamente (nunca input do
	// usuário) — Assigned/Requested são as únicas chamadoras.
	rows, err := s.db.Query(ctx, `
		SELECT `+taskCols(`t.`)+`,
		       p.id, p.name, p.color,
		       w.id, w.name,
		       c.name
		  FROM tasks t
		  JOIN projects   p ON p.id = t.project_id
		  JOIN workspaces w ON w.id = t.workspace_id
		  JOIN workspace_members wm ON wm.workspace_id = t.workspace_id AND wm.user_id = $1
		  LEFT JOIN clients c ON c.id = p.client_id
		 WHERE t.`+col+` = $1
		   AND (p.visibility = 'shared' OR p.owner_user_id = $1)
		 ORDER BY t.due_date NULLS LAST, t.created_at, t.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []AssignedTask{}
	for rows.Next() {
		var a AssignedTask
		t := &a.Task
		if err := rows.Scan(
			&t.ID, &t.WorkspaceID, &t.ProjectID, &t.ObjectiveID, &t.Name, &t.Status,
			&t.Priority, &t.NotionURL, &t.Position,
			&t.DueDate, &t.Recurrence, &t.Notes, &t.CreatedBy, &t.Assignee, &t.RequestedBy, &t.CreatedAt,
			&a.Project.ID, &a.Project.Name, &a.Project.Color,
			&a.Workspace.ID, &a.Workspace.Name,
			&a.Client,
		); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, rows.Err()
}

// Assigned devolve as tasks onde o user é o assignee, cross-workspace (CA-18).
func (s *Service) Assigned(ctx context.Context, userID string) ([]AssignedTask, error) {
	return s.assignedView(ctx, "assignee", userID)
}

// Requested devolve as tasks onde o user é o requested_by, cross-workspace (CA-19).
func (s *Service) Requested(ctx context.Context, userID string) ([]AssignedTask, error) {
	return s.assignedView(ctx, "requested_by", userID)
}
