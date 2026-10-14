// Package calendarstub — Google Calendar de mentira pro E2E da fase 06
// (TDD.md §3: nunca bater no Google real). Serve /calendars/primary/events
// com eventos ANCORADOS no timeMin recebido — assim caem sempre dentro da
// semana que o cliente pediu, independentemente do "hoje" do teste. NUNCA
// vai pra produção (só infra de teste).
package calendarstub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"
)

// Server é o Calendar falso em porta efêmera (testes Go).
type Server struct{ srv *httptest.Server }

// New sobe o stub em porta efêmera.
func New() *Server { return &Server{srv: httptest.NewServer(NewHandler())} }

// NewHandler devolve só o http.Handler — o cmd/calendarstub serve numa porta
// fixa pro E2E (injetado no backend via GOOGLE_CALENDAR_API_URL).
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /calendars/primary/events", handleEvents)
	return mux
}

func (s *Server) Close()      { s.srv.Close() }
func (s *Server) URL() string { return s.srv.URL }

func handleEvents(w http.ResponseWriter, r *http.Request) {
	anchor := time.Now()
	if v := r.URL.Query().Get("timeMin"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			anchor = t
		}
	}
	loc := anchor.Location()
	day0 := time.Date(anchor.Year(), anchor.Month(), anchor.Day(), 0, 0, 0, 0, loc)
	rfc := func(t time.Time) string { return t.Format(time.RFC3339) }
	ymd := func(t time.Time) string { return t.Format("2006-01-02") }

	// 1 evento com horário (1º dia), 1 all-day (3º dia), 1 cancelado (o
	// backend é quem filtra o cancelado — CA-08).
	body := map[string]any{
		"items": []map[string]any{
			{
				"id": "stub-reuniao", "summary": "Reunião do dia", "status": "confirmed",
				"htmlLink": "https://calendar.google.com/stub-reuniao",
				"start":    map[string]string{"dateTime": rfc(day0.Add(9 * time.Hour))},
				"end":      map[string]string{"dateTime": rfc(day0.Add(10 * time.Hour))},
			},
			{
				"id": "stub-feriado", "summary": "Feriado", "status": "confirmed",
				"htmlLink": "https://calendar.google.com/stub-feriado",
				"start":    map[string]string{"date": ymd(day0.AddDate(0, 0, 2))},
				"end":      map[string]string{"date": ymd(day0.AddDate(0, 0, 3))},
			},
			{
				"id": "stub-cancelado", "summary": "Cancelado", "status": "cancelled",
				"start": map[string]string{"dateTime": rfc(day0.Add(11 * time.Hour))},
				"end":   map[string]string{"dateTime": rfc(day0.Add(12 * time.Hour))},
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}
