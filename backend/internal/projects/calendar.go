// calendar.go — fase 28 (CA-09..15.1): itens datados do projeto (RF-21).
// Read-only, 3 queries fixas (D-05): deadline do projeto · tasks com due ∈
// janela · objetivos com target_date ∈ janela (SÓ se a coluna existir —
// CA-15.1). Gate de visibilidade pelo s.Get da fase 02 (visibilidade→404, nunca
// 500 — D-01/28-H1). NÃO retorna eventos do GCal (CA-13). NÃO importa
// objectives/tasks/calendar — queries inline (sem ciclo de import).
package projects

import (
	"context"
	"time"

	"timetrack.live/backend/internal/domain"
)

// CalendarItem é um item datado do calendário do projeto (plan §5).
type CalendarItem struct {
	Kind   string  `json:"kind"` // "task" | "objective" | "project_deadline"
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Date   string  `json:"date"` // YYYY-MM-DD
	Status *string `json:"status"`
}

// ProjectCalendarWindow resolve a janela [from,to] (CA-11): ?month=YYYY-MM →
// 1º..último dia do mês (DST-safe via AddDate); OU from/to explícitos; default
// = mês corrente no tz informado (UTC default). Malformado / to<from → 400.
func (s *Service) ProjectCalendarWindow(month, from, to, tz string) (string, string, error) {
	if month != "" {
		t, err := time.Parse("2006-01", month)
		if err != nil {
			return "", "", domain.Validation("month deve ser YYYY-MM")
		}
		first := t // 1º dia do mês (time.Parse de YYYY-MM nasce no dia 1)
		last := first.AddDate(0, 1, 0).AddDate(0, 0, -1)
		return first.Format("2006-01-02"), last.Format("2006-01-02"), nil
	}
	if from != "" || to != "" {
		fromT, err := time.Parse("2006-01-02", from)
		if err != nil {
			return "", "", domain.Validation("from deve ser YYYY-MM-DD")
		}
		toT, err := time.Parse("2006-01-02", to)
		if err != nil {
			return "", "", domain.Validation("to deve ser YYYY-MM-DD")
		}
		if toT.Before(fromT) {
			return "", "", domain.Validation("to não pode ser anterior a from")
		}
		return from, to, nil
	}
	// default: mês corrente no tz.
	loc := time.UTC
	if tz != "" {
		l, err := time.LoadLocation(tz)
		if err != nil {
			return "", "", domain.Validation("tz deve ser um fuso IANA válido")
		}
		loc = l
	}
	now := time.Now().In(loc)
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, 0).AddDate(0, 0, -1)
	return first.Format("2006-01-02"), last.Format("2006-01-02"), nil
}

// ProjectCalendar devolve os itens datados do projeto na janela [from,to]
// (inclusiva). from/to já vêm resolvidos (YYYY-MM-DD). Gate de visibilidade
// pelo s.Get (404 invisível/malformado — CA-09/12).
func (s *Service) ProjectCalendar(ctx context.Context, workspaceID, userID, projectID, from, to string) ([]CalendarItem, error) {
	// gate: visibilidade→404, uuid malformado→404, nunca 500 (D-01/28-H1).
	p, err := s.Get(ctx, workspaceID, userID, projectID)
	if err != nil {
		return nil, err
	}

	items := []CalendarItem{}

	// (a) deadline do projeto se ∈ [from,to] → kind:"project_deadline".
	if p.Deadline != nil && *p.Deadline >= from && *p.Deadline <= to {
		items = append(items, CalendarItem{
			Kind: "project_deadline", ID: p.ID, Name: p.Name, Date: *p.Deadline, Status: nil,
		})
	}

	// (b) tasks do projeto com due_date ∈ janela (inclui done — CA-10).
	taskRows, err := s.db.Query(ctx, `
		SELECT id, name, status, to_char(due_date, 'YYYY-MM-DD')
		  FROM tasks
		 WHERE workspace_id = $1 AND project_id = $2
		   AND due_date BETWEEN $3::date AND $4::date
		 ORDER BY due_date, id`, workspaceID, projectID, from, to)
	if err != nil {
		return nil, err
	}
	for taskRows.Next() {
		var it CalendarItem
		var status string
		if err := taskRows.Scan(&it.ID, &it.Name, &status, &it.Date); err != nil {
			taskRows.Close()
			return nil, err
		}
		it.Kind = "task"
		it.Status = &status
		items = append(items, it)
	}
	taskRows.Close()
	if err := taskRows.Err(); err != nil {
		return nil, err
	}

	// (c) objetivos do projeto com target_date ∈ janela — SÓ se a coluna existir
	// (CA-15.1). Coluna ausente ⇒ pula (kind:objective omitido até a fase 29).
	// Fase 29 (29-H8/CA-11): SÓ objetivos by_date viram marco — um by_tasks/
	// by_hours com target_date residual (marco dormente, CA-04.1) NÃO aparece.
	if s.hasObjectiveTargetDate {
		objRows, err := s.db.Query(ctx, `
			SELECT id, name, status, to_char(target_date, 'YYYY-MM-DD')
			  FROM objectives
			 WHERE workspace_id = $1 AND project_id = $2
			   AND completion_mode = 'by_date'
			   AND target_date BETWEEN $3::date AND $4::date
			 ORDER BY target_date, id`, workspaceID, projectID, from, to)
		if err != nil {
			return nil, err
		}
		for objRows.Next() {
			var it CalendarItem
			var status string
			if err := objRows.Scan(&it.ID, &it.Name, &status, &it.Date); err != nil {
				objRows.Close()
				return nil, err
			}
			it.Kind = "objective"
			it.Status = &status
			items = append(items, it)
		}
		objRows.Close()
		if err := objRows.Err(); err != nil {
			return nil, err
		}
	}

	return items, nil
}
