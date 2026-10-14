// Package respond escreve as respostas JSON canônicas do Timetrack.
// É um pacote folha: api, auth e os domínios das fases 02/03 importam
// daqui sem criar ciclo (api importa auth; auth nunca importa api).
package respond

import (
	"encoding/json"
	"net/http"
)

// Error escreve o envelope de erro canônico de todas as fases:
// {"error":{"code":"<slug>","message":"<humano>"}}.
func Error(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

// JSON escreve uma resposta JSON de sucesso.
func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
