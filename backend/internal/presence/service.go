// Package presence: o fan-out de presence ao vivo por workspace (fase 21). É a
// decisão arquitetural central da M2 — ver o timer ATIVO dos membros do
// workspace ao vivo. O Hub (fase 04) segue per-usuário e INTOCADO (Opção B do
// plan §1, D-01/RNF-02): o eixo de workspace vive aqui — no OnChange do user A,
// resolve os co-membros (query em workspace_members) e chama
// hub.BroadcastPresence(memberID, payload) pra cada um, per-usuário.
//
// Importa realtime só pelo BroadcastPresence (injetado como `broadcast`) e
// recebe o *pgxpool.Pool — NÃO incha o timer.Service nem o realtime.Hub (D-02,
// espelha guard/push da fase 16). Tudo é pós-commit e best-effort (CA-10): um
// erro de query é LOGADO e ENGOLIDO, nunca derruba o timer.
package presence

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/timer"
)

// PresencePayload é o corpo do envelope type:"presence" (spec §2). Por-(workspace,
// observador): o mesmo OnChange de A pode gerar payloads distintos por workspace,
// e o Project pode ser omitido pra um observador que não vê o projeto.
type PresencePayload struct {
	UserID      string     `json:"user_id"`
	Name        string     `json:"name"`
	WorkspaceID string     `json:"workspace_id"`
	Running     bool       `json:"running"`
	Project     *string    `json:"project,omitempty"` // nome — só se o viewer pode ver (shared)
	Since       *time.Time `json:"since,omitempty"`   // started_at da entry rodando; nil se !running
}

// Service resolve o fan-out de presence. Construir com NewService.
type Service struct {
	db        *pgxpool.Pool
	broadcast func(memberUserID string, payload any) // = hub.BroadcastPresence
	now       func() time.Time
}

// Option ajusta o Service na construção.
type Option func(*Service)

// WithClock injeta o relógio (testes); default time.Now.
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// NewService cria o serviço. `broadcast` é o hub.BroadcastPresence (D-04).
func NewService(db *pgxpool.Pool, broadcast func(memberUserID string, payload any), opts ...Option) *Service {
	s := &Service{db: db, broadcast: broadcast, now: time.Now}
	for _, o := range opts {
		o(s)
	}
	return s
}

// member é um destinatário do fan-out: o co-membro B e o workspace W ao qual
// este broadcast se refere.
type member struct {
	userID      string
	workspaceID string
}

// OnTimer é plugado no OnChange do timer.Service (D-04). userID=A; state=novo
// estado (nil=ocioso). Pós-commit, best-effort: panic/erro de query é LOGADO e
// ENGOLIDO (CA-10/RNF-01) — NUNCA propaga pro caminho do timer.
func (s *Service) OnTimer(userID string, state *timer.State, at time.Time) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("presence.OnTimer panic engolido (best-effort)", "user", userID, "recover", r)
		}
	}()
	if err := s.fanOut(context.Background(), userID, state); err != nil {
		slog.Error("presence.OnTimer erro engolido (best-effort)", "user", userID, "err", err)
	}
}

// fanOut resolve os destinatários + a visibilidade do projeto e empurra um
// PresencePayload pra cada (B, W). Separado pra testar o erro sem o recover.
func (s *Service) fanOut(ctx context.Context, userID string, state *timer.State) error {
	// 1) deriva running/since/projectID/entryWorkspaceID do state (sem banco).
	running, since, projectID, entryWS := derive(state)

	// nome de exibição de A (resolvido junto — RNF-01).
	nameA, err := s.userName(ctx, userID)
	if err != nil {
		return err
	}

	// 2) resolve os co-membros que devem receber o presence (CA-02/02.1/03/04):
	//    running:true → SÓ os co-membros do workspace DA ENTRY (entryWS);
	//    running:false → TODAS as memberships de A (idempotente — derruba A onde
	//    quer que o running:true tenha ido). m2.user_id<>A: A não recebe de si (D-03).
	var members []member
	if running {
		members, err = s.coMembersOfWorkspace(ctx, userID, entryWS)
	} else {
		members, err = s.coMembersAll(ctx, userID)
	}
	if err != nil {
		return err
	}

	// 3) info do projeto da entry (CA-05/06/07): resolvida UMA vez (o running:true só
	//    faz fan-out pro entryWS, onde o projeto vive). A VISIBILIDADE é decidida
	//    POR OBSERVADOR no passo 4 — NÃO se cacheia o nome já-filtrado por workspace.
	//    (Furo HIGH da verificação: um co-membro pode ser DONO de um projeto PRIVATE
	//    de A — timer.Start aceita qualquer projeto do ws sem checar dono/visibilidade
	//    — e o nome filtrado pelo 1º observador vazaria pros não-donos; o delta TEM de
	//    concordar com o Snapshot, que decide a visibilidade por linha.)
	var projInfo *projInfoT
	if running && projectID != nil {
		projInfo, err = s.projectInfo(ctx, *projectID, entryWS)
		if err != nil {
			return err
		}
	}

	// 4) empurra pra cada (B, W); o nome do projeto só vai se ESTE observador o vê.
	for _, m := range members {
		payload := PresencePayload{
			UserID:      userID,
			Name:        nameA,
			WorkspaceID: m.workspaceID,
			Running:     running,
		}
		if running {
			payload.Since = since
			if projInfo != nil && visibilityVisible(&projInfo.visibility, &projInfo.ownerUserID, m.userID) {
				n := projInfo.name
				payload.Project = &n
			}
		}
		s.broadcast(m.userID, payload)
	}
	return nil
}

// derive extrai running/since/projectID/entryWorkspaceID do state, sem tocar o
// banco (plan §3 passo 1):
//   - state==nil → running:false (ocioso/stop/corte), sem workspace.
//   - state.Mode=="break" → running:false (pausa Pomodoro), sem workspace.
//   - modo de trabalho com Entry → running:true, since/projectID/entryWS da entry.
func derive(state *timer.State) (running bool, since *time.Time, projectID *string, entryWS string) {
	if state == nil || state.Mode == "break" || state.Entry == nil {
		return false, nil, nil, ""
	}
	e := state.Entry
	st := e.StartedAt
	return true, &st, e.ProjectID, e.WorkspaceID
}

// userName resolve o nome de exibição de A.
func (s *Service) userName(ctx context.Context, userID string) (string, error) {
	var name string
	err := s.db.QueryRow(ctx, `SELECT name FROM users WHERE id = $1`, userID).Scan(&name)
	return name, err
}

// coMembersOfWorkspace resolve os co-membros de A no workspace DA ENTRY (CA-02.1):
// só esse workspace, excluindo o próprio A (D-03).
func (s *Service) coMembersOfWorkspace(ctx context.Context, userID, workspaceID string) ([]member, error) {
	rows, err := s.db.Query(ctx, `
		SELECT m2.user_id, m2.workspace_id
		  FROM workspace_members m1
		  JOIN workspace_members m2 ON m2.workspace_id = m1.workspace_id
		 WHERE m1.user_id = $1
		   AND m1.workspace_id = $2
		   AND m2.user_id <> $1`, userID, workspaceID)
	if err != nil {
		return nil, err
	}
	return scanMembers(rows)
}

// coMembersAll resolve os co-membros de A em TODAS as memberships de A (CA-04 —
// running:false derruba A em todo lugar; idempotente). m2.user_id<>A (D-03).
func (s *Service) coMembersAll(ctx context.Context, userID string) ([]member, error) {
	rows, err := s.db.Query(ctx, `
		SELECT m2.user_id, m2.workspace_id
		  FROM workspace_members m1
		  JOIN workspace_members m2 ON m2.workspace_id = m1.workspace_id
		 WHERE m1.user_id = $1
		   AND m2.user_id <> $1`, userID)
	if err != nil {
		return nil, err
	}
	return scanMembers(rows)
}

func scanMembers(rows pgx.Rows) ([]member, error) {
	defer rows.Close()
	var out []member
	for rows.Next() {
		var m member
		if err := rows.Scan(&m.userID, &m.workspaceID); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Snapshot lista os membros do workspace com timer RODANDO agora (D-05: só
// running:true), sob a regra de visibilidade do projeto pro viewer (CA-11/14).
// Isolado por e.workspace_id = $ws + join de membership (tenant — CA-14):
// uma entry rodando de um membro num OUTRO workspace dele NÃO aparece aqui.
// Visibilidade aplicada NO SERVICE (não no SQL) pra ficar idêntica ao OnTimer.
func (s *Service) Snapshot(ctx context.Context, workspaceID, viewerID string) ([]PresencePayload, error) {
	rows, err := s.db.Query(ctx, `
		SELECT e.user_id, u.name, e.started_at, e.project_id,
		       p.name, p.visibility, p.owner_user_id
		  FROM time_entries e
		  JOIN workspace_members wm ON wm.workspace_id = $1 AND wm.user_id = e.user_id
		  JOIN users u ON u.id = e.user_id
		  LEFT JOIN projects p ON p.id = e.project_id
		 WHERE e.workspace_id = $1 AND e.stopped_at IS NULL
		   AND e.user_id <> $2`, workspaceID, viewerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PresencePayload
	for rows.Next() {
		var (
			userID      string
			name        string
			startedAt   time.Time
			projectID   *string
			projName    *string
			visibility  *string
			ownerUserID *string
		)
		if err := rows.Scan(&userID, &name, &startedAt, &projectID,
			&projName, &visibility, &ownerUserID); err != nil {
			return nil, err
		}
		since := startedAt
		p := PresencePayload{
			UserID:      userID,
			Name:        name,
			WorkspaceID: workspaceID,
			Running:     true,
			Since:       &since,
		}
		// visibilidade (CA-05/06): private de outro dono → omite o nome.
		if projectID != nil && projName != nil && visibilityVisible(visibility, ownerUserID, viewerID) {
			n := *projName
			p.Project = &n
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// visibilityVisible aplica a mesma regra do visibleWhere (shared OU dono) no
// service — espelha o OnTimer pra delta e snapshot concordarem (D-05).
func visibilityVisible(visibility, ownerUserID *string, viewerID string) bool {
	if visibility == nil {
		return false
	}
	if *visibility == "shared" {
		return true
	}
	return ownerUserID != nil && *ownerUserID == viewerID
}

// projInfoT carrega o que o passo 4 do fanOut precisa do projeto pra decidir a
// visibilidade POR OBSERVADOR (espelha o per-row do Snapshot — delta=snapshot).
type projInfoT struct {
	name        string
	visibility  string
	ownerUserID string
}

// projectInfo resolve name/visibility/owner do projeto da entry SEM filtro de
// visibilidade — a decisão "este observador vê o nome?" é por membro no fanOut
// (igual ao Snapshot, via visibilityVisible). Projeto de outro ws / inexistente
// → (nil, nil): sem nome. (owner_user_id é NOT NULL na tabela projects.)
func (s *Service) projectInfo(ctx context.Context, projectID, workspaceID string) (*projInfoT, error) {
	var pi projInfoT
	err := s.db.QueryRow(ctx, `
		SELECT name, visibility, owner_user_id FROM projects
		 WHERE id = $1 AND workspace_id = $2`,
		projectID, workspaceID).Scan(&pi.name, &pi.visibility, &pi.ownerUserID)
	if err != nil {
		if isNoRows(err) {
			return nil, nil // projeto de outro ws / inexistente → omite o nome
		}
		return nil, err
	}
	return &pi, nil
}
