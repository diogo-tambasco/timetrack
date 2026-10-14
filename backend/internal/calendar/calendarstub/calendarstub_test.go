package calendarstub_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"timetrack.live/backend/internal/calendar/calendarstub"
)

// O stub ancora os eventos no timeMin pedido (sempre na semana do cliente).
func TestServeEventosAncoradosNoTimeMin(t *testing.T) {
	srv := calendarstub.New()
	defer srv.Close()

	loc, _ := time.LoadLocation("America/Sao_Paulo")
	mon := time.Date(2026, 6, 15, 0, 0, 0, 0, loc)

	resp, err := http.Get(srv.URL() + "/calendars/primary/events?timeMin=" +
		url.QueryEscape(mon.Format(time.RFC3339)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}

	var body struct {
		Items []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Start  struct {
				Date     string `json:"date"`
				DateTime string `json:"dateTime"`
			} `json:"start"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 3 {
		t.Fatalf("esperava 3 itens (1 timed, 1 all-day, 1 cancelado), veio %d", len(body.Items))
	}

	var timed, allday, cancelado bool
	for _, it := range body.Items {
		switch it.ID {
		case "stub-reuniao":
			timed = it.Start.DateTime != "" && it.Start.DateTime[:10] == "2026-06-15"
		case "stub-feriado":
			allday = it.Start.Date == "2026-06-17" // mon + 2
		case "stub-cancelado":
			cancelado = it.Status == "cancelled"
		}
	}
	if !timed {
		t.Error("evento com horário não ancorou no timeMin")
	}
	if !allday {
		t.Error("evento all-day não está em mon+2")
	}
	if !cancelado {
		t.Error("evento cancelado ausente (o backend é quem filtra)")
	}
}
