// middleware.go — RequireMember torna o isolamento multi-tenant
// estrutural (D-6): nenhum handler aninhado em /workspaces/{workspaceId}
// roda sem membership validada; não-membro e inexistente respondem o MESMO
// 404 (D-7 — não vazar existência).
package workspace

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/reqctx"
)

// RequireMember valida a membership do user autenticado no {workspaceId}
// da rota e injeta workspace_id+role no contexto.
func (s *Store) RequireMember(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := reqctx.UserID(r.Context())
		wsID := chi.URLParam(r, "workspaceId")

		var role string
		// id fora do formato uuid cai no err e vira o mesmo 404
		err := s.db.QueryRow(r.Context(), `
			SELECT role FROM workspace_members
			 WHERE workspace_id = $1 AND user_id = $2`, wsID, userID).Scan(&role)
		if err != nil {
			respond.Error(w, http.StatusNotFound, "not_found", "workspace não encontrado")
			return
		}

		ctx := reqctx.WithMembership(r.Context(), reqctx.Membership{
			WorkspaceID: wsID,
			Role:        role,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
