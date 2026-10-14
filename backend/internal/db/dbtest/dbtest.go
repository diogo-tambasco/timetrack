// Package dbtest dá aos testes de domínio um Postgres "zerado": um pool
// preso a um schema descartável do banco de teste efêmero (5433), com
// TODAS as migrações reais aplicadas. Nunca aponta pra produção (TDD.md §3).
package dbtest

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db"
)

// DSN devolve o DSN do Postgres de teste (sem skip silencioso: se o banco
// não estiver de pé, os testes FALHAM — make test-back sobe o container).
func DSN() string {
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		return dsn
	}
	return "postgres://postgres:postgres@localhost:5433/timetrack_test?sslmode=disable"
}

// NewPool abre um pool num schema isolado e migrado do banco de teste.
// O schema é dropado no fim do teste.
func NewPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	schema := fmt.Sprintf("tt_test_%d_%d", time.Now().UnixNano(), rand.Intn(1_000_000))

	admin, err := db.Connect(context.Background(), DSN(), 10*time.Second)
	if err != nil {
		t.Fatalf("banco de teste fora do ar (make test-back sobe o 5433): %v", err)
	}
	if _, err := admin.Exec(context.Background(), "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatalf("criar schema de teste: %v", err)
	}

	u, err := url.Parse(DSN())
	if err != nil {
		t.Fatalf("DSN inválido: %v", err)
	}
	q := u.Query()
	q.Set("search_path", schema+",public") // public: funções do pgcrypto
	u.RawQuery = q.Encode()

	pool, err := db.Connect(context.Background(), u.String(), 10*time.Second)
	if err != nil {
		t.Fatalf("conectar no schema de teste: %v", err)
	}
	if err := db.Migrate(context.Background(), pool, db.Migrations()); err != nil {
		t.Fatalf("migrar schema de teste: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
	})
	return pool
}

// CountRows conta as linhas de uma tabela — asserts de efeito no banco.
func CountRows(t *testing.T, pool *pgxpool.Pool, table string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatalf("contar %s: %v", table, err)
	}
	return n
}
