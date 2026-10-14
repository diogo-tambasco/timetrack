package api

import (
	"net/http"

	"timetrack.live/backend/internal/api/respond"
)

// WriteError escreve o envelope de erro canônico de todas as fases:
// {"error":{"code":"<slug>","message":"<humano>"}}.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	respond.Error(w, status, code, message)
}

// WriteJSON escreve uma resposta JSON de sucesso.
func WriteJSON(w http.ResponseWriter, status int, body any) {
	respond.JSON(w, status, body)
}
