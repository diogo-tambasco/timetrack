// Google Calendar de mentira pro E2E da fase 06: serve a Calendar API numa
// porta fixa (injetada no backend via GOOGLE_CALENDAR_API_URL). NUNCA vai
// pra produção — só infra de teste (TDD.md §3).
package main

import (
	"flag"
	"log"
	"net/http"

	"timetrack.live/backend/internal/calendar/calendarstub"
)

func main() {
	addr := flag.String("addr", ":9101", "endereço do listener")
	flag.Parse()

	log.Printf("calendarstub servindo em %s", *addr)
	if err := http.ListenAndServe(*addr, calendarstub.NewHandler()); err != nil {
		log.Fatal(err)
	}
}
