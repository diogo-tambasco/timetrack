// Google OAuth de mentira pro E2E (fase 01 T-14): serve as fixtures de
// testdata/google numa porta fixa. NUNCA vai pra produção — é só infra de
// teste (TDD.md §3: Google sempre stubbado).
package main

import (
	"flag"
	"log"
	"net/http"

	"timetrack.live/backend/internal/auth/googlestub"
)

func main() {
	addr := flag.String("addr", ":9100", "endereço do listener")
	fixtures := flag.String("fixtures", "./testdata/google", "diretório das fixtures JSON")
	flag.Parse()

	log.Printf("googlestub servindo %s em %s", *fixtures, *addr)
	if err := http.ListenAndServe(*addr, googlestub.NewHandler(*fixtures)); err != nil {
		log.Fatal(err)
	}
}
