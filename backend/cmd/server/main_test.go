package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"timetrack.live/backend/internal/config"
	"timetrack.live/backend/internal/db"
)

func testDSN() string {
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		return dsn
	}
	return "postgres://postgres:postgres@localhost:5433/timetrack_test?sslmode=disable"
}

// schemaDSN devolve um DSN apontando pra um schema zerado e descartável.
func schemaDSN(t *testing.T) string {
	t.Helper()
	schema := fmt.Sprintf("boot_test_%d_%d", time.Now().UnixNano(), rand.Intn(1_000_000))

	admin, err := db.Connect(context.Background(), testDSN(), 10*time.Second)
	if err != nil {
		t.Fatalf("banco de teste fora do ar: %v", err)
	}
	if _, err := admin.Exec(context.Background(), "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatalf("criar schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
	})

	u, err := url.Parse(testDSN())
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema+",public") // public: funções do pgcrypto
	u.RawQuery = q.Encode()
	return u.String()
}

// CA-02 (integração): quando /healthz responde 200, as migrações já rodaram
// (schema_migrations contém 0001_init e pgcrypto está instalada).
func TestBoot_MigraAntesDeServir(t *testing.T) {
	dsn := schemaDSN(t)
	cfg := config.Config{DatabaseURL: dsn, Addr: "127.0.0.1:0"}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ready := make(chan net.Addr, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, cfg, slog.New(slog.NewTextHandler(os.Stderr, nil)), Options{
			ConnectTimeout: 10 * time.Second,
			Ready:          ready,
		})
	}()

	var addr net.Addr
	select {
	case addr = <-ready:
	case err := <-errCh:
		t.Fatalf("Run morreu antes de servir: %v", err)
	case <-time.After(15 * time.Second):
		t.Fatal("timeout esperando o servidor subir")
	}

	resp, err := http.Get("http://" + addr.String() + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/healthz esperado 200, veio %d", resp.StatusCode)
	}

	pool, err := db.Connect(context.Background(), dsn, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var n int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM schema_migrations WHERE version = '0001_init'").Scan(&n); err != nil {
		t.Fatalf("ler schema_migrations: %v", err)
	}
	if n != 1 {
		t.Errorf("0001_init deveria estar registrada (veio %d)", n)
	}

	var ext int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM pg_extension WHERE extname = 'pgcrypto'").Scan(&ext); err != nil {
		t.Fatal(err)
	}
	if ext != 1 {
		t.Error("extensão pgcrypto deveria estar instalada pela 0001_init")
	}
}

// CA-04: banco indisponível → Run retorna erro com o motivo e loga a falha
// (o main converte em os.Exit(1)).
func TestRun_BancoIndisponivel_ErroComMotivo(t *testing.T) {
	cfg := config.Config{
		DatabaseURL: "postgres://postgres:postgres@localhost:59999/nada?sslmode=disable",
		Addr:        "127.0.0.1:0",
	}

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	err := Run(context.Background(), cfg, logger, Options{ConnectTimeout: 2 * time.Second})
	if err == nil {
		t.Fatal("esperava erro com banco indisponível, veio nil")
	}
	if !strings.Contains(err.Error(), "postgres") {
		t.Errorf("erro deve citar o motivo (postgres indisponível), veio: %v", err)
	}
	if !strings.Contains(logBuf.String(), "postgres") {
		t.Errorf("log deve conter a linha de falha, veio: %s", logBuf.String())
	}
}
