// Package db cuida da conexão com o Postgres (pool pgx com retry no boot)
// e do runner de migrações embutido no binário.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect abre o pool pgx tentando reconectar com backoff até o timeout
// (CA-04: o boot espera o banco por até 30s; esgotado, o processo aborta).
func Connect(ctx context.Context, dsn string, timeout time.Duration) (*pgxpool.Pool, error) {
	deadline := time.Now().Add(timeout)
	backoff := 250 * time.Millisecond

	var lastErr error
	for {
		pool, err := pgxpool.New(ctx, dsn)
		if err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			err = pool.Ping(pingCtx)
			cancel()
			if err == nil {
				return pool, nil
			}
			pool.Close()
		}
		lastErr = err

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("postgres indisponível após %s de tentativas: %w", timeout, lastErr)
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("conexão cancelada: %w", ctx.Err())
		case <-time.After(backoff):
		}
		if backoff < 2*time.Second {
			backoff *= 2
		}
	}
}
