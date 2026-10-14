// Package reqctx carrega a identidade da requisição pelo contexto:
// user autenticado (RequireAuth, fase 01) e membership do workspace
// (RequireMember). É pacote folha — auth, workspace, api e os domínios
// das fases 02/03 importam daqui sem ciclo.
package reqctx

import "context"

type ctxKey int

const (
	userIDKey ctxKey = iota
	membershipKey
)

// Membership é o vínculo validado do user com o workspace da rota.
type Membership struct {
	WorkspaceID string
	Role        string // owner | member | viewer
}

// WithUserID injeta o usuário autenticado (RequireAuth).
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// UserID devolve o usuário autenticado ("" se não houver).
func UserID(ctx context.Context) string {
	id, _ := ctx.Value(userIDKey).(string)
	return id
}

// WithMembership injeta a membership validada (RequireMember).
func WithMembership(ctx context.Context, m Membership) context.Context {
	return context.WithValue(ctx, membershipKey, m)
}

// MembershipFrom devolve a membership validada da rota (zero se ausente).
func MembershipFrom(ctx context.Context) Membership {
	m, _ := ctx.Value(membershipKey).(Membership)
	return m
}
