// Package domain define o erro de domínio comum das fases 02/03: os
// services devolvem *Error e a camada api mapeia pro envelope canônico
// {"error":{code,message}} (specs/INDEX.md). Pacote folha, como reqctx.
package domain

import "net/http"

// Error carrega o status HTTP e o code do envelope canônico.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Validation — 400 validation_error (payload fora das regras da spec).
func Validation(msg string) *Error {
	return &Error{Status: http.StatusBadRequest, Code: "validation_error", Message: msg}
}

// InvalidReference — 400 invalid_reference (referência a recurso de outro
// workspace, arquivado ou inexistente no payload).
func InvalidReference(msg string) *Error {
	return &Error{Status: http.StatusBadRequest, Code: "invalid_reference", Message: msg}
}

// NotFound — 404 not_found (inexistente E invisível respondem IGUAL,
// não vazar existência).
func NotFound(msg string) *Error {
	return &Error{Status: http.StatusNotFound, Code: "not_found", Message: msg}
}

// Forbidden — 403 forbidden (visível, mas sem permissão de escrita).
func Forbidden(msg string) *Error {
	return &Error{Status: http.StatusForbidden, Code: "forbidden", Message: msg}
}

// Conflict — 409 com code específico da fase 03 (no_active_timer,
// no_active_break, entry_running, entry_in_use).
func Conflict(code, msg string) *Error {
	return &Error{Status: http.StatusConflict, Code: code, Message: msg}
}
