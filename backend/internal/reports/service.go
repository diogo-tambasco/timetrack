// Package reports — fase 07: agregações do relatório semanal (ângulos 1–3
// do REQUISITOS §4.5). Derivado 100% de time_entries/projects/clients
// (RNF-04 — nenhuma tabela nova); janela segunda→domingo no fuso do
// cliente (D-01, padrão ParseInLocation do entries.ListDay); filtro DUPLO
// workspace+user em toda query (CA-01, constituição §8.4).
package reports

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/domain"
)

// Service calcula o relatório semanal. Clock injetável (padrão
// entries.Service) — os defaults do CA-06 dependem do "hoje" no fuso.
type Service struct {
	db  *pgxpool.Pool
	now func() time.Time
}

// Option ajusta o Service na construção.
type Option func(*Service)

// WithClock injeta o relógio (testes); default time.Now.
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// NewService cria o serviço com clock real.
func NewService(db *pgxpool.Pool, opts ...Option) *Service {
	s := &Service{db: db, now: time.Now}
	for _, o := range opts {
		o(s)
	}
	return s
}

// ProjectBucket é um item de by_project (contrato §3.1 do plan): campos de
// projeto nulos no bucket "sem projeto" (CA-10).
type ProjectBucket struct {
	ProjectID    *string `json:"project_id"`
	Name         *string `json:"name"`
	Color        *string `json:"color"`
	ClientID     *string `json:"client_id"`
	ClientName   *string `json:"client_name"`
	TotalSeconds int     `json:"total_seconds"`
}

// Focus é o ângulo 2 (foco vs. fragmentação — CA-12/13/14).
type Focus struct {
	FocusSeconds    int `json:"focus_seconds"`
	FragmentSeconds int `json:"fragment_seconds"`
	FocusCount      int `json:"focus_count"`
	FragmentCount   int `json:"fragment_count"`
	LongestSeconds  int `json:"longest_seconds"`
	Switches        int `json:"switches"`
}

// Goal é um item do ângulo 3 (meta vs. realizado — CA-15/16).
type Goal struct {
	ProjectID       string `json:"project_id"`
	Name            string `json:"name"`
	Color           string `json:"color"`
	GoalSeconds     int    `json:"goal_seconds"`
	RealizedSeconds int    `json:"realized_seconds"`
}

// Report é a resposta do GET .../reports/week.
type Report struct {
	WeekStart    string          `json:"week_start"`
	WeekEnd      string          `json:"week_end"`
	TotalSeconds int             `json:"total_seconds"`
	ByProject    []ProjectBucket `json:"by_project"`
	Focus        Focus           `json:"focus"`
	Goals        []Goal          `json:"goals"`
}

// focusThresholdSeconds — limiar INCLUSIVO de bloco de foco (25min, D-03):
// pomodoro clássico; configurável ficou fora da v1 (spec §6).
const focusThresholdSeconds = 1500

// weekWindow resolve a semana segunda→domingo que contém date no fuso
// (D-01): aritmética de CALENDÁRIO no loc (AddDate), nunca time.Hour×168
// (DST de fusos arbitrários — plan §6).
func (s *Service) weekWindow(date string, loc *time.Location) (time.Time, time.Time, error) {
	var ref time.Time
	if date == "" {
		ref = s.now().In(loc) // CA-06 default: HOJE NO FUSO
	} else {
		r, err := time.ParseInLocation("2006-01-02", date, loc)
		if err != nil {
			return time.Time{}, time.Time{}, domain.Validation("date deve ser uma data YYYY-MM-DD")
		}
		ref = r
	}
	diasDesdeSegunda := (int(ref.Weekday()) + 6) % 7 // segunda = 0
	monday := time.Date(ref.Year(), ref.Month(), ref.Day()-diasDesdeSegunda, 0, 0, 0, 0, loc)
	return monday, monday.AddDate(0, 0, 7), nil
}

// entryRow é a linha da query 1 (D-02) — base dos 3 ângulos.
type entryRow struct {
	ProjectID *string
	StartedAt time.Time
	Duration  int
}

// Week monta o relatório da semana que contém date (defaults do CA-06) pro
// usuário no workspace.
func (s *Service) Week(ctx context.Context, workspaceID, userID, date, tz string) (Report, error) {
	loc := time.UTC
	if tz != "" {
		l, err := time.LoadLocation(tz)
		if err != nil {
			return Report{}, domain.Validation("tz deve ser um fuso IANA válido")
		}
		loc = l
	}
	from, to, err := s.weekWindow(date, loc)
	if err != nil {
		return Report{}, err
	}

	// query 1 (D-02): entries FECHADAS da janela, ordem started_at ASC com
	// desempate por id (determinismo dos switches — plan §6); só entries
	// com duração (CA-04: rodando fora).
	rows, err := s.db.Query(ctx, `
		SELECT project_id, started_at, duration_seconds
		  FROM time_entries
		 WHERE workspace_id = $1 AND user_id = $2
		   AND stopped_at IS NOT NULL AND duration_seconds IS NOT NULL
		   AND started_at >= $3 AND started_at < $4
		 ORDER BY started_at ASC, id`, workspaceID, userID, from, to)
	if err != nil {
		return Report{}, err
	}
	defer rows.Close()

	list := []entryRow{}
	for rows.Next() {
		var e entryRow
		if err := rows.Scan(&e.ProjectID, &e.StartedAt, &e.Duration); err != nil {
			return Report{}, err
		}
		list = append(list, e)
	}
	if err := rows.Err(); err != nil {
		return Report{}, err
	}

	rep := Report{
		WeekStart: from.Format("2006-01-02"),
		WeekEnd:   from.AddDate(0, 0, 6).Format("2006-01-02"),
	}

	rep.ByProject, rep.TotalSeconds, err = s.byProject(ctx, workspaceID, list)
	if err != nil {
		return Report{}, err
	}
	rep.Focus = focusMetrics(list, loc)

	rep.Goals, err = s.goals(ctx, workspaceID, userID, list)
	if err != nil {
		return Report{}, err
	}
	return rep, nil
}

// goals monta o ângulo 3 (query 3 do D-02): projetos com meta, NÃO
// arquivados e VISÍVEIS pro usuário (mesma regra visibleWhere da fase 02 —
// metas de privados alheios não vazam, CA-15); realized vem da query 1
// (só a semana, só o usuário). Inclui 0 realizado — é o desvio que o
// usuário precisa ver.
func (s *Service) goals(ctx context.Context, workspaceID, userID string, list []entryRow) ([]Goal, error) {
	realized := map[string]int{}
	for _, e := range list {
		if e.ProjectID != nil {
			realized[*e.ProjectID] += e.Duration
		}
	}

	rows, err := s.db.Query(ctx, `
		SELECT id, name, color, weekly_hours_goal
		  FROM projects
		 WHERE workspace_id = $1
		   AND (visibility = 'shared' OR owner_user_id = $2)
		   AND archived_at IS NULL
		   AND weekly_hours_goal IS NOT NULL
		 ORDER BY name ASC`, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	goals := []Goal{}
	for rows.Next() {
		var g Goal
		var hours float64
		if err := rows.Scan(&g.ProjectID, &g.Name, &g.Color, &hours); err != nil {
			return nil, err
		}
		g.GoalSeconds = int(math.Round(hours * 3600)) // CA-16: fração exata, nunca truncar
		g.RealizedSeconds = realized[g.ProjectID]
		goals = append(goals, g)
	}
	return goals, rows.Err()
}

// sameProject — nil ≠ id (largar o projeto pra tarefa avulsa É troca de
// contexto — D-04); nil = nil não é troca.
func sameProject(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// focusMetrics deriva o ângulo 2 da query 1 em Go puro (D-03/D-04): limiar
// inclusivo de 1500s, longest, e switches POR DIA LOCAL (a lista já vem
// ordenada por started_at ASC com desempate por id).
func focusMetrics(list []entryRow, loc *time.Location) Focus {
	f := Focus{}
	prevDay := ""
	var prevProject *string
	for _, e := range list {
		if e.Duration >= focusThresholdSeconds {
			f.FocusSeconds += e.Duration
			f.FocusCount++
		} else {
			f.FragmentSeconds += e.Duration
			f.FragmentCount++
		}
		if e.Duration > f.LongestSeconds {
			f.LongestSeconds = e.Duration
		}
		day := e.StartedAt.In(loc).Format("2006-01-02")
		if day == prevDay && !sameProject(prevProject, e.ProjectID) {
			f.Switches++ // a 1ª entry do dia nunca conta (não há troca entre dias)
		}
		prevDay, prevProject = day, e.ProjectID
	}
	return f
}

// byProject agrega o ângulo 1: total por projeto + metadados denormalizados
// (query 2 do D-02 — SEM filtro de visibilidade, deliberado: a entry é DO
// usuário, esconder o nome do próprio histórico seria perda de dado).
func (s *Service) byProject(ctx context.Context, workspaceID string, list []entryRow) ([]ProjectBucket, int, error) {
	totals := map[string]int{} // chave "" = bucket sem projeto
	ids := []string{}
	for _, e := range list {
		key := ""
		if e.ProjectID != nil {
			key = *e.ProjectID
		}
		if _, seen := totals[key]; !seen && key != "" {
			ids = append(ids, key)
		}
		totals[key] += e.Duration
	}

	type meta struct {
		name, color string
		clientID    *string
		clientName  *string
	}
	metas := map[string]meta{}
	if len(ids) > 0 {
		rows, err := s.db.Query(ctx, `
			SELECT p.id, p.name, p.color, p.client_id, c.name
			  FROM projects p
			  LEFT JOIN clients c ON c.id = p.client_id
			 WHERE p.workspace_id = $1 AND p.id = ANY($2::uuid[])`, workspaceID, ids)
		if err != nil {
			return nil, 0, err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var m meta
			if err := rows.Scan(&id, &m.name, &m.color, &m.clientID, &m.clientName); err != nil {
				return nil, 0, err
			}
			metas[id] = m
		}
		if err := rows.Err(); err != nil {
			return nil, 0, err
		}
	}

	buckets := []ProjectBucket{}
	total := 0
	for key, sec := range totals {
		total += sec
		b := ProjectBucket{TotalSeconds: sec}
		if key != "" {
			id := key
			b.ProjectID = &id
			if m, ok := metas[key]; ok {
				name, color := m.name, m.color
				b.Name, b.Color = &name, &color
				b.ClientID, b.ClientName = m.clientID, m.clientName
			}
		}
		buckets = append(buckets, b)
	}

	// total DESC; empate: name ASC com o bucket nulo por último (§3.1)
	sort.SliceStable(buckets, func(i, j int) bool {
		a, b := buckets[i], buckets[j]
		if a.TotalSeconds != b.TotalSeconds {
			return a.TotalSeconds > b.TotalSeconds
		}
		if (a.Name == nil) != (b.Name == nil) {
			return b.Name == nil
		}
		if a.Name == nil {
			return false
		}
		return *a.Name < *b.Name
	})
	return buckets, total, nil
}
