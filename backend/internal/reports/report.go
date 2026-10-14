// report.go — fase 14: o relatório de período generalizado (CA-05..32). Reusa
// byProject/focusMetrics/goals da 07 sobre a `list` filtrada (D-03) e adiciona
// objectives (D-05), availability (D-06), focus_by_hour/kpis (D-08) e o bloco
// previous (D-04). A query 1 ganha os filtros no WHERE; o resto deriva em Go.
package reports

import (
	"context"
)

// PeriodReport é a resposta do GET .../reports (contrato §3.1 do plan).
type PeriodReport struct {
	From         string            `json:"from"`
	To           string            `json:"to"`
	TotalSeconds int               `json:"total_seconds"`
	ByProject    []ProjectBucket   `json:"by_project"`
	Focus        Focus             `json:"focus"`
	Goals        []Goal            `json:"goals"`
	Objectives   []ObjectiveItem   `json:"objectives"`
	Availability Availability      `json:"availability"`
	FocusByHour  []FocusHourBucket `json:"focus_by_hour"`
	Kpis         Kpis              `json:"kpis"`
	Previous     *PreviousReport   `json:"previous,omitempty"`
}

// PreviousReport é o bloco comparativo (CA-25): total + KPIs do período
// imediatamente anterior, mesmos filtros. Ausente sem compare (CA-26).
type PreviousReport struct {
	TotalSeconds int  `json:"total_seconds"`
	Kpis         Kpis `json:"kpis"`
}

// Report monta o relatório do período pro usuário no workspace, aplicando os
// filtros. compare=true adiciona o bloco previous (D-04). settings vem do
// report_settings (default 8h/workdays_only até a T-05 ligar a persistência).
func (s *Service) Report(ctx context.Context, workspaceID, userID string, win Window,
	filters Filters, settings Settings, compare bool) (PeriodReport, error) {

	if err := filters.validate(); err != nil { // CA-11: id malformado → 400, não 500
		return PeriodReport{}, err
	}

	list, err := s.loadEntries(ctx, workspaceID, userID, win, filters)
	if err != nil {
		return PeriodReport{}, err
	}

	rep := PeriodReport{From: win.From, To: win.To}

	rep.ByProject, rep.TotalSeconds, err = s.byProject(ctx, workspaceID, list)
	if err != nil {
		return PeriodReport{}, err
	}
	rep.Focus = focusMetrics(list, win.Loc)
	rep.Goals, err = s.goals(ctx, workspaceID, userID, list)
	if err != nil {
		return PeriodReport{}, err
	}

	rep.Objectives, err = s.objectivesSection(ctx, workspaceID, userID, win, filters)
	if err != nil {
		return PeriodReport{}, err
	}

	rep.Availability = availability(rep.TotalSeconds, win, settings)
	rep.FocusByHour = focusByHour(list, win.Loc)
	rep.Kpis = kpis(rep.TotalSeconds, rep.Focus.FocusSeconds, len(list), rep.Availability.UntrackedSeconds)

	if compare {
		prev, err := s.previous(ctx, workspaceID, userID, win, filters, settings)
		if err != nil {
			return PeriodReport{}, err
		}
		rep.Previous = &prev
	}
	return rep, nil
}

// loadEntries é a query 1 generalizada (D-03): entries FECHADAS da janela do
// período, filtro DUPLO ws+user (CA-01), só fechadas (CA-03), com os filtros
// no WHERE. Ordenada started_at ASC, id (switches determinísticos — 07).
func (s *Service) loadEntries(ctx context.Context, workspaceID, userID string,
	win Window, filters Filters) ([]entryRow, error) {

	// $1 ws, $2 user, $3 from, $4 to; os filtros começam em $5.
	sql := `
		SELECT project_id, started_at, duration_seconds
		  FROM time_entries
		 WHERE workspace_id = $1 AND user_id = $2
		   AND stopped_at IS NOT NULL AND duration_seconds IS NOT NULL
		   AND started_at >= $3 AND started_at < $4`
	args := []any{workspaceID, userID, win.Start, win.End}

	fSQL, fArgs := whereFilters(filters, 5, workspaceID, userID)
	sql += fSQL + " ORDER BY started_at ASC, id"
	args = append(args, fArgs...)

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []entryRow{}
	for rows.Next() {
		var e entryRow
		if err := rows.Scan(&e.ProjectID, &e.StartedAt, &e.Duration); err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return list, rows.Err()
}

// previous calcula o período anterior (D-04): mesma duração, deslocada um
// período atrás, mesmos filtros — só total + KPIs (CA-25). A janela anterior
// vem pré-resolvida em win.Prev (preenchida no Service antes de chamar).
func (s *Service) previous(ctx context.Context, workspaceID, userID string,
	win Window, filters Filters, settings Settings) (PreviousReport, error) {

	pwin := *win.Prev
	list, err := s.loadEntries(ctx, workspaceID, userID, pwin, filters)
	if err != nil {
		return PreviousReport{}, err
	}
	var total int
	for _, e := range list {
		total += e.Duration
	}
	f := focusMetrics(list, pwin.Loc)
	avail := availability(total, pwin, settings)
	return PreviousReport{
		TotalSeconds: total,
		Kpis:         kpis(total, f.FocusSeconds, len(list), avail.UntrackedSeconds),
	}, nil
}
