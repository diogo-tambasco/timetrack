// notes_handlers.go — REST das notas nativas (fase 26, plan §4). Tradução fina:
// visibilidade, gate de escrita por-nota e a atomicidade do slash moram no
// notes.Service. As rotas são montadas num subrouter IRMÃO do workspace SEM
// RequireWriteRole (plan §3 D-02): o gate de escrita é condicional por nota
// (member que enxerga LÊ; só escreve com access=edit/owner), não a regra
// binária "viewer nunca escreve" — espelha exatamente comments (fase 19).
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/notes"
	"timetrack.live/backend/internal/reqctx"
)

// Notes agrega o service de notas (fase 26).
type Notes struct{ Svc *notes.Service }

// WithNotes liga as rotas de notas no router.
func WithNotes(n Notes) Option {
	return func(cfg *routerCfg) {
		cfg.notes = &n
	}
}

func (n *Notes) handleList(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	// ?project_id= vazio → notas soltas do user (CA-03.1).
	list, err := n.Svc.List(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()),
		r.URL.Query().Get("project_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"notes": list})
}

func (n *Notes) handleGet(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	note, err := n.Svc.Get(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()),
		chi.URLParam(r, "noteId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, note)
}

func (n *Notes) handleCreate(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	var body struct {
		ProjectID *string `json:"project_id"`
		TaskID    *string `json:"task_id"`
		Title     string  `json:"title"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	note, err := n.Svc.Create(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		notes.CreateInput{ProjectID: body.ProjectID, TaskID: body.TaskID, Title: body.Title})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, note)
}

func (n *Notes) handlePatch(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	// tri-state via json.RawMessage: nil=ausente (mantém), valor=define. body_md
	// aceita '' (limpa). Espelha decodeTriState do tasks_handlers (fase 24).
	var body struct {
		Title  json.RawMessage `json:"title"`
		BodyMD json.RawMessage `json:"body_md"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	in := notes.PatchInput{}
	if !decodeTriState(w, body.Title, "title inválido", &in.TitleSet, &in.Title) {
		return
	}
	if !decodeTriState(w, body.BodyMD, "body_md inválido", &in.BodySet, &in.BodyMD) {
		return
	}
	note, err := n.Svc.Patch(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		chi.URLParam(r, "noteId"), in)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, note)
}

func (n *Notes) handleDelete(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	if err := n.Svc.Delete(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		chi.URLParam(r, "noteId")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (n *Notes) handleCreateLink(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	var body struct {
		Kind  string `json:"kind"`
		Name  string `json:"name"`
		Title string `json:"title"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	// /task usa "name", /note usa "title" no contrato (plan §4.2); aceitamos
	// qualquer um que venha preenchido (o service valida o nome/título).
	name := body.Name
	if name == "" {
		name = body.Title
	}
	res, err := n.Svc.CreateLink(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		chi.URLParam(r, "noteId"), body.Kind, name)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, res)
}

func (n *Notes) handleDeleteLink(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	if err := n.Svc.DeleteLink(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		chi.URLParam(r, "noteId"), chi.URLParam(r, "linkId")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
