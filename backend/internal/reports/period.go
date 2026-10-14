// period.go — fase 14: generaliza a janela da 07 (weekWindow) pra período
// arbitrário (day/week/month/custom) e os filtros (projeto/cliente/objetivo).
// A 07 NÃO é tocada (RNF-05): weekWindow/byProject/focusMetrics/goals seguem
// donos do /reports/week. Aqui mora a aritmética de calendário no fuso
// (AddDate, nunca time.Hour×N — DST de fusos arbitrários) e os predicados de
// filtro parametrizados (D-02/D-03).
package reports

import (
	"strconv"
	"time"

	"timetrack.live/backend/internal/domain"
)

// Window é a janela resolvida do período: [Start, End) no fuso Loc, com as
// datas (YYYY-MM-DD) que o cliente ecoa como rótulo (CA-08). Days é a duração
// em dias (End − Start contado por calendário) — base do availability (D-06).
type Window struct {
	Start time.Time
	End   time.Time
	Loc   *time.Location
	From  string  // data resolvida (YYYY-MM-DD) — rótulo
	To    string  // data resolvida INCLUSIVA (YYYY-MM-DD) — rótulo
	Prev  *Window // janela imediatamente anterior (compare=previous, D-04)
}

// loadLoc resolve o fuso IANA; vazio → UTC; inválido → 400 (CA-07).
func loadLoc(tz string) (*time.Location, error) {
	if tz == "" {
		return time.UTC, nil
	}
	l, err := time.LoadLocation(tz)
	if err != nil {
		return nil, domain.Validation("tz deve ser um fuso IANA válido")
	}
	return l, nil
}

// ResolveWindow é a entrada pública: resolve a janela do período (D-02) e
// anexa a janela anterior (D-04) pra quando compare=previous for pedido. tz
// vazio → UTC; malformados → 400 (CA-07).
func (s *Service) ResolveWindow(period, date, from, to, tz string) (Window, error) {
	loc, err := loadLoc(tz)
	if err != nil {
		return Window{}, err
	}
	win, err := s.periodWindow(period, date, from, to, loc)
	if err != nil {
		return Window{}, err
	}
	custom := from != "" || to != ""
	prev := previousWindow(win, period, custom)
	win.Prev = &prev
	return win, nil
}

// periodWindow resolve a janela do período (D-02): from/to explícitos (custom)
// PRECEDEM period (CA-06); senão period (day/week/month) ancorado em date (ou
// hoje no fuso). Tudo por AddDate no loc (calendário, imune a DST). Validação:
// datas/period/tz malformados ou to<from → 400 (CA-07).
func (s *Service) periodWindow(period, date, from, to string, loc *time.Location) (Window, error) {
	// custom: from+to explícitos precedem period (CA-06).
	if from != "" || to != "" {
		if from == "" || to == "" {
			return Window{}, domain.Validation("from e to devem ser passados juntos")
		}
		f, err := time.ParseInLocation("2006-01-02", from, loc)
		if err != nil {
			return Window{}, domain.Validation("from deve ser uma data YYYY-MM-DD")
		}
		t, err := time.ParseInLocation("2006-01-02", to, loc)
		if err != nil {
			return Window{}, domain.Validation("to deve ser uma data YYYY-MM-DD")
		}
		if t.Before(f) {
			return Window{}, domain.Validation("to deve ser maior ou igual a from")
		}
		end := t.AddDate(0, 0, 1) // [from 00:00, to+1d 00:00) — to INCLUSIVO (CA-05)
		return Window{Start: f, End: end, Loc: loc,
			From: f.Format("2006-01-02"), To: t.Format("2006-01-02")}, nil
	}

	// referência do preset: date ou hoje no fuso (CA-06 defaults).
	var ref time.Time
	if date == "" {
		ref = s.now().In(loc)
	} else {
		r, err := time.ParseInLocation("2006-01-02", date, loc)
		if err != nil {
			return Window{}, domain.Validation("date deve ser uma data YYYY-MM-DD")
		}
		ref = r
	}

	switch period {
	case "", "week":
		// reusa a janela provada da 07 (segunda→domingo) — CA-09.
		start, end, err := s.weekWindow(date, loc)
		if err != nil {
			return Window{}, err
		}
		return Window{Start: start, End: end, Loc: loc,
			From: start.Format("2006-01-02"),
			To:   start.AddDate(0, 0, 6).Format("2006-01-02")}, nil
	case "day":
		start := time.Date(ref.Year(), ref.Month(), ref.Day(), 0, 0, 0, 0, loc)
		end := start.AddDate(0, 0, 1)
		return Window{Start: start, End: end, Loc: loc,
			From: start.Format("2006-01-02"), To: start.Format("2006-01-02")}, nil
	case "month":
		start := time.Date(ref.Year(), ref.Month(), 1, 0, 0, 0, 0, loc)
		end := start.AddDate(0, 1, 0) // 1º do mês seguinte (calendário)
		return Window{Start: start, End: end, Loc: loc,
			From: start.Format("2006-01-02"),
			To:   end.AddDate(0, 0, -1).Format("2006-01-02")}, nil
	default:
		return Window{}, domain.Validation("period deve ser day, week ou month")
	}
}

// previousWindow desloca a janela um período inteiro pra trás, de mesmo
// tamanho (D-04): week −7d; month addMonths −1 (clamp do dia via 1º do mês);
// day −1d; custom −duração (em dias de calendário). Reancorada no MESMO loc.
func previousWindow(win Window, period string, custom bool) Window {
	loc := win.Loc
	if custom {
		// custom: janela de mesmo tamanho terminando no dia ANTES de from.
		nDays := daysBetween(win.Start, win.End)
		prevEnd := win.Start // [prevStart, from)
		prevStart := prevEnd.AddDate(0, 0, -nDays)
		return Window{Start: prevStart, End: prevEnd, Loc: loc,
			From: prevStart.Format("2006-01-02"),
			To:   prevEnd.AddDate(0, 0, -1).Format("2006-01-02")}
	}
	switch period {
	case "day":
		start := win.Start.AddDate(0, 0, -1)
		return Window{Start: start, End: win.Start, Loc: loc,
			From: start.Format("2006-01-02"), To: start.Format("2006-01-02")}
	case "month":
		start := win.Start.AddDate(0, -1, 0) // 1º do mês anterior (calendário; clamp do dia trivial)
		return Window{Start: start, End: win.Start, Loc: loc,
			From: start.Format("2006-01-02"),
			To:   win.Start.AddDate(0, 0, -1).Format("2006-01-02")}
	default: // week (e default)
		start := win.Start.AddDate(0, 0, -7)
		return Window{Start: start, End: win.Start, Loc: loc,
			From: start.Format("2006-01-02"),
			To:   start.AddDate(0, 0, 6).Format("2006-01-02")}
	}
}

// daysBetween conta dias de calendário entre start e end no loc de start
// (AddDate +1d até alcançar end), imune a DST (não usa end.Sub/24h — D-06).
func daysBetween(start, end time.Time) int {
	n := 0
	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		n++
	}
	return n
}

// Filters são os filtros opcionais da query 1 (D-03): combináveis (CA-11),
// ausentes = sem filtro. Aplicados no WHERE da query que enche a list.
type Filters struct {
	ProjectID   string
	ClientID    string
	ObjectiveID string
}

// validate checa que os ids passados são uuid canônico; malformado → 400
// (CA-11). Vazio = ausente (sem filtro).
func (f Filters) validate() error {
	for _, id := range []string{f.ProjectID, f.ClientID, f.ObjectiveID} {
		if id != "" && !isUUID(id) {
			return domain.Validation("filtro deve ser um uuid válido")
		}
	}
	return nil
}

// whereFilters monta os predicados de filtro parametrizados sobre a tabela
// time_entries (alias implícito — colunas project_id/task_id) a partir do
// índice de placeholder inicial (D-03). project_id direto; client_id via
// projetos VISÍVEIS do cliente; objective_id via tasks do objetivo de projeto
// VISÍVEL. O visibleWhere (= projeto shared OU do usuário) garante que um id
// de outro usuário/privado resulte em conjunto vazio, não vaze (CA-11/12).
//
// userID entra como $idx pro visibleWhere; os ids de filtro entram em seguida.
// Retorna o SQL e os args na ordem.
func whereFilters(f Filters, startIdx int, workspaceID, userID string) (string, []any) {
	sql := ""
	args := []any{}
	idx := startIdx

	if f.ProjectID != "" {
		sql += " AND project_id = $" + strconv.Itoa(idx)
		args = append(args, f.ProjectID)
		idx++
	}
	if f.ClientID != "" {
		// entries cujos projetos pertencem ao cliente E são visíveis ao usuário.
		sql += " AND project_id IN (SELECT id FROM projects" +
			" WHERE workspace_id = $" + strconv.Itoa(idx) +
			" AND client_id = $" + strconv.Itoa(idx+1) +
			" AND (visibility = 'shared' OR owner_user_id = $" + strconv.Itoa(idx+2) + "))"
		args = append(args, workspaceID, f.ClientID, userID)
		idx += 3
	}
	if f.ObjectiveID != "" {
		// entries cujas tasks pertencem ao objetivo, cujo projeto é visível.
		sql += " AND task_id IN (SELECT t.id FROM tasks t" +
			" JOIN projects p ON p.id = t.project_id" +
			" WHERE t.workspace_id = $" + strconv.Itoa(idx) +
			" AND t.objective_id = $" + strconv.Itoa(idx+1) +
			" AND (p.visibility = 'shared' OR p.owner_user_id = $" + strconv.Itoa(idx+2) + "))"
		args = append(args, workspaceID, f.ObjectiveID, userID)
		idx += 3
	}
	return sql, args
}

// isUUID — forma canônica 8-4-4-4-12 (hex), como a fase 03/02. Evita 500 do
// Postgres em filtro malformado (vira 400 antes).
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
		if !isHex {
			return false
		}
	}
	return true
}
