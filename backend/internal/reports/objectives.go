// objectives.go — fase 14: progresso de objetivos por ENTREGA (CA-16..19/16.1,
// D-05). Lê as tabelas da fase 12 (objectives + tasks.objective_id) — a 14 só
// SELECT. 3 queries: (q-obj) objetivos de projetos VISÍVEIS com atividade-no-
// período OU meta, restritos pelos filtros ativos (CA-16.1); (q-tasks)
// contagem CUMULATIVA de tasks por objetivo (sem janela — entrega é cumulativa,
// CA-17); (q-realized) tempo DO PERÍODO por objetivo (filtro duplo ws+user).
// Composição em Go: progress_pct = round(done/total×100) (espelha donePct da 12).
package reports

import (
	"context"
	"math"
	"sort"
	"strconv"
)

// ObjectiveItem é um item da seção objectives (contrato §3.1). GoalSeconds = 0
// e omitido quando o objetivo não tem goal_hours. Fase 29 (CA-06.3): CompletionMode
// + IsComplete derivado POR MODO (by_hours = tracked cumulativo per-usuário >= meta;
// by_tasks/by_date = status persistido) — um by_hours da meta-batida com status
// 'open' aparece concluído, sem mentir "Open".
type ObjectiveItem struct {
	ProjectID       string `json:"project_id"`
	ProjectName     string `json:"project_name"`
	ProjectColor    string `json:"project_color"`
	ObjectiveID     string `json:"objective_id"`
	ObjectiveName   string `json:"objective_name"`
	Status          string `json:"status"`
	CompletionMode  string `json:"completion_mode"` // fase 29 (CA-06.3)
	IsComplete      bool   `json:"is_complete"`     // fase 29: concluído POR MODO
	TasksTotal      int    `json:"tasks_total"`
	TasksDone       int    `json:"tasks_done"`
	ProgressPct     int    `json:"progress_pct"`
	RealizedSeconds int    `json:"realized_seconds"`
	GoalSeconds     int    `json:"goal_seconds,omitempty"`
}

// objectivesSection monta a lista de objetivos por entrega (D-05). Vazia (nunca
// nil) quando não há objetivo elegível.
func (s *Service) objectivesSection(ctx context.Context, workspaceID, userID string,
	win Window, filters Filters) ([]ObjectiveItem, error) {

	// q-realized: tempo DO PERÍODO por objetivo (via task_id → objective_id),
	// filtro duplo ws+user, só fechadas. NÃO aplica os filtros aqui — a janela
	// vale pro tempo; a LISTA é restrita pela q-obj (CA-16.1/17).
	realized := map[string]int{}
	rRows, err := s.db.Query(ctx, `
		SELECT t.objective_id, COALESCE(SUM(e.duration_seconds), 0)
		  FROM time_entries e
		  JOIN tasks t ON t.id = e.task_id
		 WHERE e.workspace_id = $1 AND e.user_id = $2
		   AND e.stopped_at IS NOT NULL AND e.duration_seconds IS NOT NULL
		   AND e.started_at >= $3 AND e.started_at < $4
		   AND t.objective_id IS NOT NULL
		 GROUP BY t.objective_id`, workspaceID, userID, win.Start, win.End)
	if err != nil {
		return nil, err
	}
	for rRows.Next() {
		var objID string
		var sum int
		if err := rRows.Scan(&objID, &sum); err != nil {
			rRows.Close()
			return nil, err
		}
		realized[objID] = sum
	}
	rRows.Close()
	if err := rRows.Err(); err != nil {
		return nil, err
	}

	// q-tracked-cum (fase 29 — CA-06.3): horas CUMULATIVAS per-usuário por
	// objetivo (sem janela, MESMO bucket do /board) pra derivar "concluído by_hours"
	// (tracked >= meta), independente do status persistido (que pode estar 'open' —
	// CA-06.2). NÃO substitui o realized do período (q-realized acima).
	trackedCum := map[string]int{}
	cRows, err := s.db.Query(ctx, `
		SELECT t.objective_id, COALESCE(SUM(e.duration_seconds), 0)
		  FROM time_entries e
		  JOIN tasks t ON t.id = e.task_id
		 WHERE e.workspace_id = $1 AND e.user_id = $2
		   AND e.duration_seconds IS NOT NULL
		   AND t.objective_id IS NOT NULL
		 GROUP BY t.objective_id`, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	for cRows.Next() {
		var objID string
		var sum int
		if err := cRows.Scan(&objID, &sum); err != nil {
			cRows.Close()
			return nil, err
		}
		trackedCum[objID] = sum
	}
	cRows.Close()
	if err := cRows.Err(); err != nil {
		return nil, err
	}

	// q-tasks: contagem CUMULATIVA (total/done) por objetivo (CA-17 — sem
	// janela). Só dos objetivos de projetos do ws (o JOIN garante).
	type counts struct{ total, done int }
	tasksByObj := map[string]counts{}
	tRows, err := s.db.Query(ctx, `
		SELECT objective_id,
		       COUNT(*),
		       COUNT(*) FILTER (WHERE status = 'done')
		  FROM tasks
		 WHERE workspace_id = $1 AND objective_id IS NOT NULL
		 GROUP BY objective_id`, workspaceID)
	if err != nil {
		return nil, err
	}
	for tRows.Next() {
		var objID string
		var c counts
		if err := tRows.Scan(&objID, &c.total, &c.done); err != nil {
			tRows.Close()
			return nil, err
		}
		tasksByObj[objID] = c
	}
	tRows.Close()
	if err := tRows.Err(); err != nil {
		return nil, err
	}

	// q-obj: objetivos de projetos VISÍVEIS com atividade-no-período OU meta,
	// restritos pelos filtros ativos (CA-16.1). O ramo "OU goal_hours" NÃO
	// sobrepõe o filtro: os whereFilters da q (via predicados de projeto/
	// objetivo) restringem a lista.
	sql := `
		SELECT o.id, o.name, o.status, o.goal_hours, o.completion_mode,
		       p.id, p.name, p.color
		  FROM objectives o
		  JOIN projects p ON p.id = o.project_id
		 WHERE o.workspace_id = $1
		   AND (p.visibility = 'shared' OR p.owner_user_id = $2)`
	args := []any{workspaceID, userID}

	// filtros: restringem a LISTA de objetivos (CA-16.1).
	if filters.ProjectID != "" {
		sql += " AND o.project_id = $" + strconv.Itoa(len(args)+1)
		args = append(args, filters.ProjectID)
	}
	if filters.ClientID != "" {
		sql += " AND p.client_id = $" + strconv.Itoa(len(args)+1)
		args = append(args, filters.ClientID)
	}
	if filters.ObjectiveID != "" {
		sql += " AND o.id = $" + strconv.Itoa(len(args)+1)
		args = append(args, filters.ObjectiveID)
	}

	// atividade-no-período (objetivo com realized > 0) OU meta. As chaves de
	// realized vêm da q-realized acima; aqui só precisamos do "tem meta".
	sql += " ORDER BY p.name ASC, o.name ASC"

	oRows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer oRows.Close()

	out := []ObjectiveItem{}
	for oRows.Next() {
		var it ObjectiveItem
		var goalHours *float64
		if err := oRows.Scan(&it.ObjectiveID, &it.ObjectiveName, &it.Status, &goalHours, &it.CompletionMode,
			&it.ProjectID, &it.ProjectName, &it.ProjectColor); err != nil {
			return nil, err
		}
		_, hasActivity := realized[it.ObjectiveID]
		hasGoal := goalHours != nil
		// CA-16: entra se teve atividade no período OU tem meta. Sob filtro,
		// a q já restringiu a lista; mas o ramo "atividade OU meta" ainda
		// decide se um objetivo SEM nenhum dos dois aparece (não deve).
		if !hasActivity && !hasGoal {
			continue
		}
		c := tasksByObj[it.ObjectiveID]
		it.TasksTotal = c.total
		it.TasksDone = c.done
		it.ProgressPct = donePctReports(c.done, c.total)
		it.RealizedSeconds = realized[it.ObjectiveID]
		if hasGoal {
			it.GoalSeconds = int(math.Round(*goalHours * 3600))
		}
		// fase 29 (CA-06.3): "concluído" POR MODO. by_hours = tracked cumulativo
		// per-usuário >= meta (independente do status persistido — CA-06.2) OU já
		// fechado manualmente; by_tasks/by_date = status persistido. Nunca mostra
		// um by_hours da meta-batida como "Open".
		it.IsComplete = isObjectiveComplete(it.CompletionMode, it.Status, hasGoal, goalHours, trackedCum[it.ObjectiveID])
		out = append(out, it)
	}
	if err := oRows.Err(); err != nil {
		return nil, err
	}

	// ordem já vem do SQL (project_name ASC, objective_name ASC); estabiliza
	// por id em empate exato pra determinismo.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ProjectName != out[j].ProjectName {
			return out[i].ProjectName < out[j].ProjectName
		}
		if out[i].ObjectiveName != out[j].ObjectiveName {
			return out[i].ObjectiveName < out[j].ObjectiveName
		}
		return out[i].ObjectiveID < out[j].ObjectiveID
	})
	return out, nil
}

// donePctReports = round(done/total×100); 0 quando total=0 (CA-16, espelha o
// donePct da fase 12, board.go).
func donePctReports(done, total int) int {
	if total == 0 {
		return 0
	}
	return int(math.Round(float64(done) / float64(total) * 100))
}

// isObjectiveComplete deriva "concluído" POR MODO (fase 29 — CA-06.3). by_hours:
// a meta de horas batida (tracked cumulativo per-usuário >= goal_hours×3600)
// conta como concluído INDEPENDENTE do status persistido (que pode estar 'open'
// — CA-06.2); ou já fechado manualmente. by_tasks/by_date: status persistido.
func isObjectiveComplete(mode, status string, hasGoal bool, goalHours *float64, trackedCumSeconds int) bool {
	if status == "done" {
		return true
	}
	if mode == "by_hours" && hasGoal {
		return float64(trackedCumSeconds) >= *goalHours*3600
	}
	return false
}
