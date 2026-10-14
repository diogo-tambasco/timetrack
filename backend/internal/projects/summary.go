// summary.go — fase 28 (CA-04..08, RNF-04/05): cards vivos da lista de
// projetos. 1 request, 3 queries fixas (D-02), montadas em memória O(n) — sem
// N+1. tracked_seconds = bucket "projeto" do user (D-03, = /board); due_today
// no fuso da requisição (D-04). Visibilidade e isolamento de tenant em TODA
// query (CA-04.1/09).
package projects

import (
	"context"
	"time"

	"timetrack.live/backend/internal/domain"
)

// ProjectSummary é a linha de card vivo (plan §5).
type ProjectSummary struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Color          string  `json:"color"`
	ClientID       *string `json:"client_id"`
	Deadline       *string `json:"deadline"`
	TaskCount      int     `json:"task_count"`
	DoneCount      int     `json:"done_count"`
	OpenCount      int     `json:"open_count"`
	TrackedSeconds int64   `json:"tracked_seconds"`
	DueTodayCount  int     `json:"due_today_count"`
}

// Summary devolve os contadores leves de TODOS os projetos visíveis do
// workspace (CA-04). tz vazio → UTC; inválido → 400 (CA-06).
func (s *Service) Summary(ctx context.Context, workspaceID, userID, tz string, archived bool) ([]ProjectSummary, error) {
	loc := time.UTC
	if tz != "" {
		l, err := time.LoadLocation(tz)
		if err != nil {
			return nil, domain.Validation("tz deve ser um fuso IANA válido")
		}
		loc = l
	}
	today := time.Now().In(loc).Format("2006-01-02")

	// (1) projetos visíveis do ws (CA-04.1/08): shared OU do próprio user;
	// arquivados só com o flag. ORDER BY igual à listagem (created_at, id).
	rows, err := s.db.Query(ctx, `
		SELECT id, name, color, client_id, to_char(deadline, 'YYYY-MM-DD')
		  FROM projects
		 WHERE workspace_id = $1 AND (visibility = 'shared' OR owner_user_id = $2)
		   AND ($3 OR archived_at IS NULL)
		 ORDER BY created_at, id`, workspaceID, userID, archived)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []ProjectSummary{}
	idx := map[string]int{}
	for rows.Next() {
		var ps ProjectSummary
		if err := rows.Scan(&ps.ID, &ps.Name, &ps.Color, &ps.ClientID, &ps.Deadline); err != nil {
			return nil, err
		}
		idx[ps.ID] = len(list)
		list = append(list, ps)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return list, nil
	}

	// (2) contadores de task por projeto (CA-04/05): só projetos visíveis do ws.
	// due_today = due_date = hoje no tz E status != 'done'.
	cntRows, err := s.db.Query(ctx, `
		SELECT t.project_id,
		       count(*),
		       count(*) FILTER (WHERE t.status = 'done'),
		       count(*) FILTER (WHERE t.due_date = $3::date AND t.status <> 'done')
		  FROM tasks t
		  JOIN projects p ON p.id = t.project_id
		 WHERE t.workspace_id = $1
		   AND p.workspace_id = $1 AND (p.visibility = 'shared' OR p.owner_user_id = $2)
		 GROUP BY t.project_id`, workspaceID, userID, today)
	if err != nil {
		return nil, err
	}
	for cntRows.Next() {
		var pid string
		var total, done, dueToday int
		if err := cntRows.Scan(&pid, &total, &done, &dueToday); err != nil {
			cntRows.Close()
			return nil, err
		}
		if i, ok := idx[pid]; ok {
			list[i].TaskCount = total
			list[i].DoneCount = done
			list[i].OpenCount = total - done
			list[i].DueTodayCount = dueToday
		}
	}
	cntRows.Close()
	if err := cntRows.Err(); err != nil {
		return nil, err
	}

	// (3) soma de tempo por projeto (D-03): entries FECHADAS do user com
	// project_id = projeto (incluindo as sem task_id). Só projetos visíveis.
	timeRows, err := s.db.Query(ctx, `
		SELECT e.project_id, SUM(e.duration_seconds)
		  FROM time_entries e
		  JOIN projects p ON p.id = e.project_id
		 WHERE e.workspace_id = $1 AND e.user_id = $2 AND e.duration_seconds IS NOT NULL
		   AND p.workspace_id = $1 AND (p.visibility = 'shared' OR p.owner_user_id = $2)
		 GROUP BY e.project_id`, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	for timeRows.Next() {
		var pid string
		var sum int64
		if err := timeRows.Scan(&pid, &sum); err != nil {
			timeRows.Close()
			return nil, err
		}
		if i, ok := idx[pid]; ok {
			list[i].TrackedSeconds = sum
		}
	}
	timeRows.Close()
	if err := timeRows.Err(); err != nil {
		return nil, err
	}

	return list, nil
}
