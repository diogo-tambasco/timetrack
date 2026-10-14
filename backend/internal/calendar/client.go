// client.go — chamada à Google Calendar API v3 (agenda primary) e
// mapeamento do JSON do Google pro Event do app. Base URL injetável (stub
// em teste, RNF-01). NÃO loga token/Authorization (RNF-02).
package calendar

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"timetrack.live/backend/internal/domain"
)

// Event é um evento da agenda principal já mapeado pro app (CA-05/07).
type Event struct {
	ID       string    `json:"id"`
	Summary  string    `json:"summary"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	AllDay   bool      `json:"all_day"`
	HTMLLink string    `json:"html_link"`
}

// rawWhen é o start/end do Google: `dateTime` (RFC3339, com horário) OU
// `date` (YYYY-MM-DD, all-day sem fuso).
type rawWhen struct {
	Date     string `json:"date"`
	DateTime string `json:"dateTime"`
}

type rawEvent struct {
	ID       string  `json:"id"`
	Summary  string  `json:"summary"`
	Status   string  `json:"status"`
	HTMLLink string  `json:"htmlLink"`
	Start    rawWhen `json:"start"`
	End      rawWhen `json:"end"`
}

// upstream — 502 calendar_upstream (CA-10.1): falha do Google que não é
// reautenticação. Nunca 500.
func upstream(msg string) *domain.Error {
	return &domain.Error{Status: http.StatusBadGateway, Code: "calendar_upstream", Message: msg}
}

// listPrimary chama a Calendar API da agenda `primary` na janela [from, to)
// e mapeia os eventos. O fuso de `from` resolve datas de eventos all-day
// (que o Google manda sem fuso).
func (s *Service) listPrimary(ctx context.Context, accessToken string, from, to time.Time) ([]Event, error) {
	loc := from.Location()

	q := url.Values{}
	q.Set("timeMin", from.Format(time.RFC3339))
	q.Set("timeMax", to.Format(time.RFC3339))
	q.Set("singleEvents", "true") // expande recorrências
	q.Set("orderBy", "startTime")
	q.Set("maxResults", "2500")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		s.apiBase+"/calendars/primary/events?"+q.Encode(), nil)
	if err != nil {
		return nil, upstream("montar requisição ao Google")
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.httpc.Do(req)
	if err != nil {
		return nil, upstream("falha de rede ao Google")
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// segue
	case http.StatusUnauthorized:
		// token rejeitado mesmo após refresh → reconectar (CA-10)
		return nil, domain.Conflict("calendar_reconnect_required", "reconecte o Google Calendar")
	default:
		return nil, upstream("Google respondeu " + resp.Status)
	}

	var body struct {
		Items []rawEvent `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, upstream("corpo inválido do Google")
	}

	events := make([]Event, 0, len(body.Items))
	for _, it := range body.Items {
		if it.Status == "cancelled" { // CA-08
			continue
		}
		ev := Event{ID: it.ID, Summary: it.Summary, HTMLLink: it.HTMLLink}
		switch {
		case it.Start.DateTime != "":
			st, e1 := time.Parse(time.RFC3339, it.Start.DateTime)
			en, e2 := time.Parse(time.RFC3339, it.End.DateTime)
			if e1 != nil {
				continue // sem início parseável: ignora o evento, não derruba a leitura
			}
			if e2 != nil {
				en = st
			}
			ev.Start, ev.End, ev.AllDay = st, en, false
		case it.Start.Date != "": // CA-07: all-day, sem fuso → resolve no loc
			st, e1 := time.ParseInLocation("2006-01-02", it.Start.Date, loc)
			if e1 != nil {
				continue
			}
			en, e2 := time.ParseInLocation("2006-01-02", it.End.Date, loc)
			if e2 != nil {
				en = st.AddDate(0, 0, 1) // fim exclusivo do Google ausente → 1 dia
			}
			ev.Start, ev.End, ev.AllDay = st, en, true
		default:
			continue
		}
		events = append(events, ev)
	}
	return events, nil
}
