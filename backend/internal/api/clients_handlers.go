// clients_handlers.go — REST de clientes (plan §3.1, CA-01..05). Handlers
// finos: regra de negócio mora em internal/clients; o workspace_id vem
// SEMPRE da membership validada (CA-26).
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/reqctx"
)

func jsonDecode(r *http.Request, dst any) error {
	return json.NewDecoder(r.Body).Decode(dst)
}

func (c *Core) handleListClients(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	list, err := c.Clients.List(r.Context(), m.WorkspaceID, r.URL.Query().Get("archived") == "true")
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"clients": list})
}

func (c *Core) handleCreateClient(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	var body struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	created, err := c.Clients.Create(r.Context(), m.WorkspaceID, body.Name)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, created)
}

func (c *Core) handlePatchClient(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	var body struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	updated, err := c.Clients.Rename(r.Context(), m.WorkspaceID, chi.URLParam(r, "clientId"), body.Name)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, updated)
}

func (c *Core) handleDeleteClient(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	if err := c.Clients.Archive(r.Context(), m.WorkspaceID, chi.URLParam(r, "clientId")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
