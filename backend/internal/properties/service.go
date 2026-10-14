// service.go — fase 25 T-04: CRUD de defs de propriedade + set/clear de value por
// task. Validação na mão (RNF-01); gate de projeto interno (gate.go). Toda query nova
// carrega workspace_id + project_id da rota (CA-12/15). `type` imutável no PATCH (D-04).
package properties

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"timetrack.live/backend/internal/domain"
)

// PropertyDef é o SCHEMA de uma propriedade custom de um projeto (CA-01). Option
// vive em validate.go (quem consulta o id no select).
type PropertyDef struct {
	ID          string   `json:"id"`
	ProjectID   string   `json:"project_id"`
	WorkspaceID string   `json:"workspace_id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Options     []Option `json:"options"`
	Position    int      `json:"position"`
}

// PropertyValue é o valor de uma def numa task (jsonb tipado conforme def.Type).
type PropertyValue struct {
	DefID string          `json:"def_id"`
	Value json.RawMessage `json:"value"`
}

// CreateDefInput é o payload de criação (POST). `type` define o schema.
type CreateDefInput struct {
	Name    string
	Type    string
	Options []Option
}

// PatchDefInput é o payload de renomear/reordenar/editar options (PATCH). `type`
// NÃO entra (imutável — D-04).
type PatchDefInput struct {
	Name        *string
	PositionSet bool
	Position    *int
	OptionsSet  bool
	Options     []Option
}

const defCols = `id, project_id, workspace_id, name, type, options, position`

// validV1Type são os 6 tipos liberados na v1 (CA-02/D-09). person/multi_select são
// backlog (a fatia 25.1 liga só o service — o schema/CHECK já comporta).
func validV1Type(t string) bool {
	switch t {
	case "text", "number", "select", "date", "url", "checkbox":
		return true
	}
	return false
}

// validName aplica a regra de nome da def (CA-01.1): 1–80 chars após trim.
func validName(name string) (string, error) {
	n := strings.TrimSpace(name)
	if n == "" || len([]rune(n)) > 80 {
		return "", domain.Validation("nome inválido (1–80 caracteres)")
	}
	return n, nil
}

// normalizeOptions valida/normaliza options (CA-03). type ≠ select → [] (options
// ignorado). select → não-vazio, cada label 1–40 (trim), sem label duplicado; gera
// id faltante, preserva id existente (CA-05/D-08).
func normalizeOptions(typ string, in []Option) ([]Option, error) {
	if typ != "select" {
		return []Option{}, nil
	}
	if len(in) == 0 {
		return nil, domain.Validation("select exige ao menos uma opção")
	}
	seen := map[string]bool{}
	out := make([]Option, 0, len(in))
	for _, o := range in {
		label := strings.TrimSpace(o.Label)
		if label == "" || len([]rune(label)) > 40 {
			return nil, domain.Validation("opção com label inválido (1–40 caracteres)")
		}
		if seen[label] {
			return nil, domain.Validation("opção com label duplicado")
		}
		seen[label] = true
		id := strings.TrimSpace(o.ID)
		if id == "" {
			id = uuid.NewString()
		}
		out = append(out, Option{ID: id, Label: label, Color: o.Color})
	}
	return out, nil
}

func unmarshalOptions(raw []byte) []Option {
	out := []Option{}
	if len(raw) == 0 {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	if out == nil {
		out = []Option{}
	}
	return out
}

// ListDefs devolve as defs do projeto ordenadas por position (CA-04). Gate de
// leitura: projeto invisível → 404 (CA-15).
func (s *Service) ListDefs(ctx context.Context, ws, userID, projectID string) ([]PropertyDef, error) {
	if _, err := s.loadProjectGate(ctx, ws, userID, projectID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT `+defCols+`
		  FROM project_property_defs
		 WHERE workspace_id = $1 AND project_id = $2
		 ORDER BY position, id`, ws, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	defs := []PropertyDef{}
	for rows.Next() {
		var d PropertyDef
		var raw []byte
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.WorkspaceID, &d.Name, &d.Type, &raw, &d.Position); err != nil {
			return nil, err
		}
		d.Options = unmarshalOptions(raw)
		defs = append(defs, d)
	}
	return defs, rows.Err()
}

// CreateDef cria uma def no fim da lista (CA-01/02/03/07). Validação ANTES do gate e
// do SQL; escrita em projeto arquivado → 400 invalid_reference.
func (s *Service) CreateDef(ctx context.Context, ws, userID, role, projectID string, in CreateDefInput) (PropertyDef, error) {
	name, err := validName(in.Name)
	if err != nil {
		return PropertyDef{}, err
	}
	if !validV1Type(in.Type) {
		return PropertyDef{}, domain.Validation("tipo de propriedade inválido")
	}
	opts, err := normalizeOptions(in.Type, in.Options)
	if err != nil {
		return PropertyDef{}, err
	}
	if _, err := s.ensureWrite(ctx, ws, userID, role, projectID, true); err != nil {
		return PropertyDef{}, err
	}
	optsJSON, _ := json.Marshal(opts)
	var d PropertyDef
	var raw []byte
	err = s.db.QueryRow(ctx, `
		INSERT INTO project_property_defs (project_id, workspace_id, name, type, options, position)
		VALUES ($1, $2, $3, $4, $5::jsonb,
		        COALESCE((SELECT max(position) + 1 FROM project_property_defs WHERE project_id = $1), 0))
		RETURNING `+defCols,
		projectID, ws, name, in.Type, string(optsJSON)).Scan(
		&d.ID, &d.ProjectID, &d.WorkspaceID, &d.Name, &d.Type, &raw, &d.Position)
	if err != nil {
		return PropertyDef{}, err
	}
	d.Options = unmarshalOptions(raw)
	return d, nil
}

// getDef carrega uma def casando (workspace, projeto da rota, id) — CA-12/15. def de
// outro projeto/workspace → 404 (não vaza). uuid malformado → 404 (nunca 500).
func (s *Service) getDef(ctx context.Context, ws, projectID, defID string) (PropertyDef, error) {
	if uuid.Validate(defID) != nil {
		return PropertyDef{}, domain.NotFound("propriedade não encontrada")
	}
	var d PropertyDef
	var raw []byte
	err := s.db.QueryRow(ctx, `
		SELECT `+defCols+`
		  FROM project_property_defs
		 WHERE workspace_id = $1 AND project_id = $2 AND id = $3`,
		ws, projectID, defID).Scan(
		&d.ID, &d.ProjectID, &d.WorkspaceID, &d.Name, &d.Type, &raw, &d.Position)
	if errors.Is(err, pgx.ErrNoRows) {
		return PropertyDef{}, domain.NotFound("propriedade não encontrada")
	}
	if err != nil {
		return PropertyDef{}, err
	}
	d.Options = unmarshalOptions(raw)
	return d, nil
}

// PatchDef persiste o subconjunto presente de {name, position, options} (CA-05).
// `type` é imutável (ignorado). Escrita em projeto arquivado → 400.
func (s *Service) PatchDef(ctx context.Context, ws, userID, role, projectID, defID string, in PatchDefInput) (PropertyDef, error) {
	if _, err := s.ensureWrite(ctx, ws, userID, role, projectID, true); err != nil {
		return PropertyDef{}, err
	}
	cur, err := s.getDef(ctx, ws, projectID, defID)
	if err != nil {
		return PropertyDef{}, err
	}
	if in.Name != nil {
		n, err := validName(*in.Name)
		if err != nil {
			return PropertyDef{}, err
		}
		cur.Name = n
	}
	if in.PositionSet && in.Position != nil {
		cur.Position = *in.Position
	}
	if in.OptionsSet {
		opts, err := normalizeOptions(cur.Type, in.Options)
		if err != nil {
			return PropertyDef{}, err
		}
		cur.Options = opts
	}
	optsJSON, _ := json.Marshal(cur.Options)
	var d PropertyDef
	var raw []byte
	err = s.db.QueryRow(ctx, `
		UPDATE project_property_defs
		   SET name = $4, position = $5, options = $6::jsonb
		 WHERE workspace_id = $1 AND project_id = $2 AND id = $3
		RETURNING `+defCols,
		ws, projectID, defID, cur.Name, cur.Position, string(optsJSON)).Scan(
		&d.ID, &d.ProjectID, &d.WorkspaceID, &d.Name, &d.Type, &raw, &d.Position)
	if err != nil {
		return PropertyDef{}, err
	}
	d.Options = unmarshalOptions(raw)
	return d, nil
}

// DeleteDef remove a def (hard delete) e seus values via CASCADE (CA-06). Nenhuma
// task é apagada. blockArchived=false (espelha objectives.Delete: deletar schema de
// projeto arquivado é permitido).
func (s *Service) DeleteDef(ctx context.Context, ws, userID, role, projectID, defID string) error {
	if _, err := s.ensureWrite(ctx, ws, userID, role, projectID, false); err != nil {
		return err
	}
	if _, err := s.getDef(ctx, ws, projectID, defID); err != nil {
		return err
	}
	_, err := s.db.Exec(ctx,
		`DELETE FROM project_property_defs WHERE workspace_id = $1 AND project_id = $2 AND id = $3`,
		ws, projectID, defID)
	return err
}

// taskInProject confirma que a task pertence ao (workspace, projeto da rota) — CA-12.
func (s *Service) taskInProject(ctx context.Context, ws, projectID, taskID string) error {
	if uuid.Validate(taskID) != nil {
		return domain.NotFound("task não encontrada")
	}
	var ok bool
	err := s.db.QueryRow(ctx,
		`SELECT true FROM tasks WHERE workspace_id = $1 AND project_id = $2 AND id = $3`,
		ws, projectID, taskID).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NotFound("task não encontrada")
	}
	return err
}

// isJSONNull detecta `null` cru (CA-09.1, padrão tasks_handlers.go).
func isJSONNull(raw json.RawMessage) bool {
	return string(bytes.TrimSpace(raw)) == "null"
}

// SetValue faz upsert do value (CA-08) após validar pelo type da def (CA-09).
// `value:null` (qualquer tipo) delega pra ClearValue (CA-09.1) e devolve cleared=true.
// Retorna o value normalizado (cleared=false) ou (nil, true) no clear. Escrita em
// projeto arquivado → 400; def/task de outro projeto/workspace → 404 (CA-11/12/15).
func (s *Service) SetValue(ctx context.Context, ws, userID, role, projectID, taskID, defID string, raw json.RawMessage) (json.RawMessage, bool, error) {
	if isJSONNull(raw) {
		if err := s.ClearValue(ctx, ws, userID, role, projectID, taskID, defID); err != nil {
			return nil, false, err
		}
		return nil, true, nil
	}
	if uuid.Validate(taskID) != nil {
		return nil, false, domain.NotFound("task não encontrada")
	}
	if _, err := s.ensureWrite(ctx, ws, userID, role, projectID, true); err != nil {
		return nil, false, err
	}
	def, err := s.getDef(ctx, ws, projectID, defID)
	if err != nil {
		return nil, false, err
	}
	if err := s.taskInProject(ctx, ws, projectID, taskID); err != nil {
		return nil, false, err
	}
	norm, err := validateValue(def.Type, def.Options, raw)
	if err != nil {
		return nil, false, err
	}
	if _, err := s.db.Exec(ctx, `
		INSERT INTO task_property_values (task_id, def_id, workspace_id, value)
		VALUES ($1, $2, $3, $4::jsonb)
		ON CONFLICT (task_id, def_id) DO UPDATE SET value = EXCLUDED.value, workspace_id = EXCLUDED.workspace_id`,
		taskID, defID, ws, string(norm)); err != nil {
		return nil, false, err
	}
	return norm, false, nil
}

// ClearValue remove a linha de value (CA-10), idempotente (204 mesmo sem value).
// Escrita em projeto arquivado → 400 (CA-11); def/task de outro projeto → 404 (CA-12).
func (s *Service) ClearValue(ctx context.Context, ws, userID, role, projectID, taskID, defID string) error {
	if uuid.Validate(taskID) != nil {
		return domain.NotFound("task não encontrada")
	}
	if _, err := s.ensureWrite(ctx, ws, userID, role, projectID, true); err != nil {
		return err
	}
	if _, err := s.getDef(ctx, ws, projectID, defID); err != nil {
		return err
	}
	if err := s.taskInProject(ctx, ws, projectID, taskID); err != nil {
		return err
	}
	_, err := s.db.Exec(ctx,
		`DELETE FROM task_property_values WHERE workspace_id = $1 AND task_id = $2 AND def_id = $3`,
		ws, taskID, defID)
	return err
}
