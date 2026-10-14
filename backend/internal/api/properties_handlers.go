// properties_handlers.go — fase 25 T-06: REST de defs de propriedade + set/clear de
// value (handlers finos; a regra mora no properties.Service). Rotas aninhadas no
// projeto/task sob a cadeia RequireAuth→RequireMember→RequireWriteRole (CA-14).
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/properties"
	"timetrack.live/backend/internal/reqctx"
)

func (c *Core) handleListPropertyDefs(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	defs, err := c.Properties.ListDefs(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()),
		chi.URLParam(r, "projectId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"properties": defs})
}

func (c *Core) handleCreatePropertyDef(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	var body struct {
		Name    string              `json:"name"`
		Type    string              `json:"type"`
		Options []properties.Option `json:"options"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	d, err := c.Properties.CreateDef(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		chi.URLParam(r, "projectId"), properties.CreateDefInput{Name: body.Name, Type: body.Type, Options: body.Options})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, d)
}

func (c *Core) handlePatchPropertyDef(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	// `type` NÃO entra (imutável — D-04): não é campo do body.
	var body struct {
		Name     *string              `json:"name"`
		Position *int                 `json:"position"`
		Options  *[]properties.Option `json:"options"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	in := properties.PatchDefInput{Name: body.Name}
	if body.Position != nil {
		in.PositionSet = true
		in.Position = body.Position
	}
	if body.Options != nil {
		in.OptionsSet = true
		in.Options = *body.Options
	}
	d, err := c.Properties.PatchDef(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		chi.URLParam(r, "projectId"), chi.URLParam(r, "defId"), in)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, d)
}

func (c *Core) handleDeletePropertyDef(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	if err := c.Properties.DeleteDef(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		chi.URLParam(r, "projectId"), chi.URLParam(r, "defId")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *Core) handleSetPropertyValue(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	var body struct {
		Value json.RawMessage `json:"value"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Value == nil { // campo value ausente (≠ null, que é clear — CA-09.1)
		respond.Error(w, http.StatusBadRequest, "validation_error", "value ausente")
		return
	}
	val, cleared, err := c.Properties.SetValue(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		chi.URLParam(r, "projectId"), chi.URLParam(r, "taskId"), chi.URLParam(r, "defId"), body.Value)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if cleared { // value:null → clear → 204 (CA-09.1)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"def_id": chi.URLParam(r, "defId"), "value": val})
}

func (c *Core) handleClearPropertyValue(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	if err := c.Properties.ClearValue(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		chi.URLParam(r, "projectId"), chi.URLParam(r, "taskId"), chi.URLParam(r, "defId")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
