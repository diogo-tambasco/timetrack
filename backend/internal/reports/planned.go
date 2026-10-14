// planned.go — fase 06 ângulo 4 (Planejado × Real). Planejado = eventos do
// GCal COM horário, por dia local do start (all-day fora — CA-13.1). Real =
// time entries fechadas, por dia local do started_at (mesmo bucket dos
// switches da fase 07). A janela é a MESMA weekWindow do /reports/week
// (CA-13/15); reports NÃO importa calendar — o source é uma interface
// consumidora satisfeita pelo adaptador na camada api.
package reports

import (
	"context"
	"time"

	"timetrack.live/backend/internal/domain"
)

// PlannedEvent é o evento do calendar como o ângulo 4 precisa.
type PlannedEvent struct {
	Start  time.Time
	End    time.Time
	AllDay bool
}

// PlannedSource é a fonte de eventos planejados (o calendar.Service satisfaz
// via adaptador na api). Definida AQUI (consumidor) pra evitar ciclo de import.
type PlannedSource interface {
	Connected(ctx context.Context, userID string) (bool, error)
	Events(ctx context.Context, userID string, from, to time.Time) ([]PlannedEvent, error)
}

// PlannedDay é um dia da semana no ângulo 4.
type PlannedDay struct {
	Date            string `json:"date"`
	PlannedSeconds  int    `json:"planned_seconds"`
	RealizedSeconds int    `json:"realized_seconds"`
}

// PlannedReport é a resposta do GET .../reports/planned.
type PlannedReport struct {
	WeekStart string       `json:"week_start"`
	WeekEnd   string       `json:"week_end"`
	Connected bool         `json:"connected"`
	Days      []PlannedDay `json:"days"`
}

// RealizedByDay soma a duração das entries FECHADAS por dia local do
// started_at, na janela [from, to). Filtro DUPLO workspace+user (CA-01).
func (s *Service) RealizedByDay(ctx context.Context, workspaceID, userID string, from, to time.Time, loc *time.Location) (map[string]int, error) {
	rows, err := s.db.Query(ctx, `
		SELECT started_at, duration_seconds
		  FROM time_entries
		 WHERE workspace_id = $1 AND user_id = $2
		   AND stopped_at IS NOT NULL AND duration_seconds IS NOT NULL
		   AND started_at >= $3 AND started_at < $4`, workspaceID, userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byDay := map[string]int{}
	for rows.Next() {
		var started time.Time
		var dur int
		if err := rows.Scan(&started, &dur); err != nil {
			return nil, err
		}
		byDay[started.In(loc).Format("2006-01-02")] += dur
	}
	return byDay, rows.Err()
}

// PlannedReport monta o ângulo 4 da semana que contém date no fuso tz. Não
// conectado → connected:false, planned 0, realizado preenchido (CA-14), sem
// chamar o Google.
func (s *Service) PlannedReport(ctx context.Context, workspaceID, userID, date, tz string, src PlannedSource) (PlannedReport, error) {
	loc := time.UTC
	if tz != "" {
		l, err := time.LoadLocation(tz)
		if err != nil {
			return PlannedReport{}, domain.Validation("tz deve ser um fuso IANA válido")
		}
		loc = l
	}
	from, to, err := s.weekWindow(date, loc) // [segunda 00:00, segunda+7d) — idêntica ao /reports/week
	if err != nil {
		return PlannedReport{}, err
	}

	realized, err := s.RealizedByDay(ctx, workspaceID, userID, from, to, loc)
	if err != nil {
		return PlannedReport{}, err
	}

	connected, err := src.Connected(ctx, userID)
	if err != nil {
		return PlannedReport{}, err
	}

	planned := map[string]int{}
	if connected {
		events, err := src.Events(ctx, userID, from, to)
		if err != nil {
			// Verificação do build: token revogado / Google fora NÃO pode
			// derrubar o relatório nem sumir com o realizado (dado LOCAL, já
			// calculado acima). Degrada como "não conectado" → o front mostra
			// o CTA de reconexão e o realizado segue visível nos ângulos 1–3.
			connected = false
		} else {
			for _, e := range events {
				if e.AllDay { // CA-13.1: all-day não é tempo planejado de trabalho
					continue
				}
				day := e.Start.In(loc).Format("2006-01-02")
				dur := int(e.End.Sub(e.Start).Seconds())
				if dur < 0 {
					dur = 0
				}
				planned[day] += dur // eventos fora dos 7 dias caem em chave ignorada (CA-15)
			}
		}
	}

	rep := PlannedReport{
		WeekStart: from.Format("2006-01-02"),
		WeekEnd:   from.AddDate(0, 0, 6).Format("2006-01-02"), // domingo INCLUSIVO (= /reports/week)
		Connected: connected,
		Days:      make([]PlannedDay, 0, 7),
	}
	for i := 0; i < 7; i++ {
		day := from.AddDate(0, 0, i).Format("2006-01-02")
		rep.Days = append(rep.Days, PlannedDay{
			Date:            day,
			PlannedSeconds:  planned[day],
			RealizedSeconds: realized[day],
		})
	}
	return rep, nil
}
