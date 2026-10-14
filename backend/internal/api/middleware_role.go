// middleware_role.go — role gate da fase 02 (D-02): único middleware NOVO
// da cadeia; sessão (RequireAuth) e membership (RequireMember) vêm prontos
// da fase 01.
package api

import (
	"net/http"
	"regexp"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/reqctx"
)

// patchSingleTaskPath casa EXATAMENTE PATCH /api/v1/workspaces/{id}/tasks/{taskId}
// (um único segmento final após /tasks/) — o bypass do carve-out viewer-assignee
// (fase 18, D-03). Ancorado nos dois extremos pra NÃO casar subrotas mais fundas
// (ex.: comentários da fase 19 sob a task). Não usa o RoutePattern do chi porque,
// dentro de um Use no subrouter /workspaces/{workspaceId}, ele não expõe o padrão
// aninhado de forma confiável.
var patchSingleTaskPath = regexp.MustCompile(`^/api/v1/workspaces/[^/]+/tasks/[^/]+$`)

// RequireWriteRole barra viewer em métodos de escrita (CA-23) num ponto
// único do subrouter do workspace; leitura passa direto (CA-24).
//
// Fase 18 (D-03): UM buraco condicional — o viewer passa em PATCH de UMA task
// (e SÓ aí); o service (tasks.Patch) é a autoridade final (viewer-assignee +
// só-status). Qualquer outro método/recurso/rota segue 403 pro viewer. A
// assinatura do middleware NÃO muda (CA-24).
func RequireWriteRole(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete:
			if reqctx.MembershipFrom(r.Context()).Role == "viewer" {
				// bypass ESTREITO: PATCH de uma task individual → deixa o service
				// decidir (carve-out viewer-assignee, fase 18 CA-10/11).
				if r.Method == http.MethodPatch && patchSingleTaskPath.MatchString(r.URL.Path) {
					break
				}
				respond.Error(w, http.StatusForbidden, "forbidden",
					"viewer não tem permissão de escrita")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// RequireOwner (fase 17, CA-21) barra quem não é owner do workspace da rota em
// QUALQUER método — gate mais estrito que o RequireWriteRole (que só barra
// viewer em escrita). Aplicado SÓ ao grupo de invitations/members; roda depois
// do RequireMember (404 não-membro antes do 403 — CA-21.1).
func RequireOwner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reqctx.MembershipFrom(r.Context()).Role != "owner" {
			respond.Error(w, http.StatusForbidden, "forbidden",
				"só o dono gere membros e convites")
			return
		}
		next.ServeHTTP(w, r)
	})
}
