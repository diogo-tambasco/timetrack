package db

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool abre um pool num schema isolado e descartável do banco de teste,
// pra cada teste enxergar um "banco zerado" sem interferir nos vizinhos.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	schema := fmt.Sprintf("mig_test_%d_%d", time.Now().UnixNano(), rand.Intn(1_000_000))

	admin, err := Connect(context.Background(), testDSN(t), 10*time.Second)
	if err != nil {
		t.Fatalf("banco de teste fora do ar: %v", err)
	}
	if _, err := admin.Exec(context.Background(), "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatalf("criar schema de teste: %v", err)
	}

	u, err := url.Parse(testDSN(t))
	if err != nil {
		t.Fatalf("DSN inválido: %v", err)
	}
	q := u.Query()
	q.Set("search_path", schema+",public") // public: funções do pgcrypto
	u.RawQuery = q.Encode()

	pool, err := Connect(context.Background(), u.String(), 10*time.Second)
	if err != nil {
		t.Fatalf("conectar no schema de teste: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
	})
	return pool
}

func appliedVersions(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		"SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		t.Fatalf("ler schema_migrations: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			t.Fatal(err)
		}
		out = append(out, v)
	}
	return out
}

// CA-02: aplica em ordem crescente de NNNN (0002 depende de 0001 — só
// funciona se 0001 rodar primeiro).
func TestMigrate_AplicaEmOrdem(t *testing.T) {
	pool := testPool(t)
	fsys := fstest.MapFS{
		"0002_insere.sql": {Data: []byte("INSERT INTO t1 (x) VALUES (42);")},
		"0001_cria.sql":   {Data: []byte("CREATE TABLE t1 (x int);")},
	}

	if err := Migrate(context.Background(), pool, fsys); err != nil {
		t.Fatalf("migrate falhou: %v", err)
	}

	var x int
	if err := pool.QueryRow(context.Background(), "SELECT x FROM t1").Scan(&x); err != nil {
		t.Fatalf("t1 deveria existir com 1 linha: %v", err)
	}
	if x != 42 {
		t.Errorf("esperava 42, veio %d", x)
	}
	got := appliedVersions(t, pool)
	if len(got) != 2 || got[0] != "0001_cria" || got[1] != "0002_insere" {
		t.Errorf("schema_migrations esperado [0001_cria 0002_insere], veio %v", got)
	}
}

// CA-03: rodar 2x não reaplica nada — estado idêntico, 1 registro por versão.
func TestMigrate_Idempotente_NaoReaplica(t *testing.T) {
	pool := testPool(t)
	fsys := fstest.MapFS{
		// INSERT sem WHERE: se reaplicasse, haveria 2 linhas
		"0001_cria.sql": {Data: []byte("CREATE TABLE t1 (x int); INSERT INTO t1 (x) VALUES (1);")},
	}

	if err := Migrate(context.Background(), pool, fsys); err != nil {
		t.Fatalf("1ª rodada falhou: %v", err)
	}
	if err := Migrate(context.Background(), pool, fsys); err != nil {
		t.Fatalf("2ª rodada falhou (deveria ser no-op): %v", err)
	}

	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM t1").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("migração reaplicada: t1 tem %d linhas, esperava 1", n)
	}
	if got := appliedVersions(t, pool); len(got) != 1 {
		t.Errorf("schema_migrations deveria ter 1 registro, tem %v", got)
	}
}

// CA-05: migração com SQL inválido aborta e NÃO consta em schema_migrations.
func TestMigrate_SQLInvalido_AbortaSemRegistrar(t *testing.T) {
	pool := testPool(t)
	fsys := fstest.MapFS{
		"0001_ok.sql":       {Data: []byte("CREATE TABLE t1 (x int);")},
		"0002_quebrada.sql": {Data: []byte("CREATE TABEL oops;")},
	}

	err := Migrate(context.Background(), pool, fsys)
	if err == nil {
		t.Fatal("esperava erro com SQL inválido, veio nil")
	}

	got := appliedVersions(t, pool)
	if len(got) != 1 || got[0] != "0001_ok" {
		t.Errorf("só 0001_ok deveria estar registrada, veio %v", got)
	}
}

// Proteção contra typo: nome fora do padrão NNNN_nome.sql → erro de boot.
func TestMigrate_NomeForaDoPadrao_Erra(t *testing.T) {
	pool := testPool(t)
	fsys := fstest.MapFS{
		"001_curta.sql": {Data: []byte("SELECT 1;")},
	}

	if err := Migrate(context.Background(), pool, fsys); err == nil {
		t.Fatal("esperava erro com nome fora do padrão NNNN_nome.sql, veio nil")
	}
}
