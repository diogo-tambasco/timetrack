// handlers.go — GET /api/v1/workspaces/{workspaceId} (§3.5): a rota
// mínima da fase 01 que materializa o isolamento; as fases 02/03 penduram
// sub-rotas no mesmo subrouter.
package workspace

import (
	"net/http"
	"time"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/reqctx"
)

// HandleGetWorkspace responde o workspace da rota com a role do user e a
// lista de membros (extensão aditiva do §3.5 — fonte da seção "Membros do
// projeto" do modal da fase 02, CA-28). Roda sempre atrás do RequireMember
// (membership já validada — a query de membros é escopada pelo workspace
// da rota, CA-26).
func (s *Store) HandleGetWorkspace(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())

	var name string
	var createdAt time.Time
	if err := s.db.QueryRow(r.Context(), `
		SELECT name, created_at FROM workspaces WHERE id = $1`, m.WorkspaceID).
		Scan(&name, &createdAt); err != nil {
		respond.Error(w, http.StatusNotFound, "not_found", "workspace não encontrado")
		return
	}

	rows, err := s.db.Query(r.Context(), `
		SELECT u.id, u.name, u.email, wm.role
		  FROM workspace_members wm
		  JOIN users u ON u.id = wm.user_id
		 WHERE wm.workspace_id = $1
		 ORDER BY (wm.role = 'owner') DESC, u.name`, m.WorkspaceID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal", "erro ao listar membros")
		return
	}
	defer rows.Close()

	type memberJSON struct {
		UserID string `json:"user_id"`
		Name   string `json:"name"`
		Email  string `json:"email"`
		Role   string `json:"role"`
	}
	members := []memberJSON{}
	for rows.Next() {
		var mb memberJSON
		if err := rows.Scan(&mb.UserID, &mb.Name, &mb.Email, &mb.Role); err != nil {
			respond.Error(w, http.StatusInternalServerError, "internal", "erro ao listar membros")
			return
		}
		members = append(members, mb)
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"workspace": map[string]any{
			"id":         m.WorkspaceID,
			"name":       name,
			"role":       m.Role,
			"created_at": createdAt.UTC().Format(time.RFC3339),
			"members":    members,
		},
	})
}
