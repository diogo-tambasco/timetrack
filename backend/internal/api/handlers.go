package api

import "net/http"

// handleHealthz — CA-01: liveness simples; se o processo está de pé, as
// migrações já rodaram (CA-02).
func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
