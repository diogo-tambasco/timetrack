// Package push: Web Push (VAPID) da fase 16 — persistência das subscriptions
// de navegador (Subscriptions) e o envio best-effort do aviso de corte
// (Sender). Toda gravação/leitura é por user_id da sessão (isolamento — CA-13).
package push

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Subscription é o trio que o navegador entrega no subscribe (CA-13). UserID
// fica fora: a gravação é sempre em nome do usuário da sessão.
type Subscription struct {
	Endpoint string `json:"endpoint"`
	P256dh   string `json:"p256dh"`
	Auth     string `json:"auth"`
}

// Subscriptions é o repositório de push_subscriptions. Construir com NewSubscriptions.
type Subscriptions struct {
	db *pgxpool.Pool
}

// NewSubscriptions cria o repositório.
func NewSubscriptions(db *pgxpool.Pool) *Subscriptions {
	return &Subscriptions{db: db}
}

// Upsert persiste a subscription sob o userID, com upsert por endpoint (CA-13):
// re-inscrever o MESMO navegador substitui as chaves e reatribui o user_id (o
// endpoint é do device atual — se trocou de conta, a inscrição vira da nova).
func (r *Subscriptions) Upsert(ctx context.Context, userID string, s Subscription) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO push_subscriptions (endpoint, user_id, p256dh, auth)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (endpoint) DO UPDATE
		   SET user_id = excluded.user_id,
		       p256dh  = excluded.p256dh,
		       auth    = excluded.auth`,
		s.Endpoint, userID, s.P256dh, s.Auth)
	return err
}

// ListByUser devolve as subscriptions do usuário (isolamento — CA-13/CA-8
// análogo: o filtro WHERE user_id é obrigatório).
func (r *Subscriptions) ListByUser(ctx context.Context, userID string) ([]Subscription, error) {
	rows, err := r.db.Query(ctx, `
		SELECT endpoint, p256dh, auth FROM push_subscriptions
		 WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	subs := []Subscription{}
	for rows.Next() {
		var s Subscription
		if err := rows.Scan(&s.Endpoint, &s.P256dh, &s.Auth); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

// Delete remove a subscription por endpoint (limpeza de sub morta — CA-16). O
// endpoint é único (PK), então não precisa de user_id pra apontar a linha.
func (r *Subscriptions) Delete(ctx context.Context, endpoint string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM push_subscriptions WHERE endpoint = $1`, endpoint)
	return err
}
