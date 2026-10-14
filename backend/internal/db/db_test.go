package db

import (
	"context"
	"os"
	"testing"
	"time"
)

// testDSN devolve o DSN do Postgres efêmero de teste (porta 5433 — D-13).
// Falha o teste se o banco de teste não estiver de pé: sem skip silencioso.
func testDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5433/timetrack_test?sslmode=disable"
	}
	return dsn
}

// CA-04: banco indisponível → retry até o timeout e erro (boot aborta).
func TestConnect_BancoIndisponivel_FalhaAposTimeout(t *testing.T) {
	deadDSN := "postgres://postgres:postgres@localhost:59999/nada?sslmode=disable"

	start := time.Now()
	_, err := Connect(context.Background(), deadDSN, 2*time.Second)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("esperava erro com banco indisponível, veio nil")
	}
	if elapsed < 1*time.Second {
		t.Errorf("deveria ter tentado reconectar até o timeout (~2s), desistiu em %v", elapsed)
	}
	if elapsed > 10*time.Second {
		t.Errorf("timeout de 2s não foi respeitado: levou %v", elapsed)
	}
}

func TestConnect_BancoDePe_Conecta(t *testing.T) {
	pool, err := Connect(context.Background(), testDSN(t), 10*time.Second)
	if err != nil {
		t.Fatalf("esperava conectar no banco de teste (5433 de pé?): %v", err)
	}
	defer pool.Close()

	var um int
	if err := pool.QueryRow(context.Background(), "SELECT 1").Scan(&um); err != nil {
		t.Fatalf("SELECT 1 falhou: %v", err)
	}
	if um != 1 {
		t.Errorf("SELECT 1 retornou %d", um)
	}
}
