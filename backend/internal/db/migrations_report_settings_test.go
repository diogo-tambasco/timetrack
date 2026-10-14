package db

import (
	"context"
	"testing"
)

// Fase 14 T-06 — TestMigracaoReportSettings: preferências do relatório POR
// USUÁRIO (plan §2). Defaults 8/true na linha; CHECK barra work_hours_per_day
// fora de (0,24]. PK em user_id (1 config por usuário). Espelha
// calendar_settings (fase 11).
func TestMigracaoReportSettings(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool, Migrations()); err != nil {
		t.Fatalf("migrações reais falharam: %v", err)
	}

	var ok bool
	if err := pool.QueryRow(ctx,
		"SELECT to_regclass('report_settings') IS NOT NULL").Scan(&ok); err != nil || !ok {
		t.Fatalf("tabela report_settings não existe (err=%v)", err)
	}

	var userA, userB string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-rs-a', 'rsa@b.c', 'Ana') RETURNING id`).Scan(&userA); err != nil {
		t.Fatalf("insert user A: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-rs-b', 'rsb@b.c', 'Bia') RETURNING id`).Scan(&userB); err != nil {
		t.Fatalf("insert user B: %v", err)
	}

	// insert sem valores aplica os defaults 8/true (CA-21/29 backstop)
	var hpd float64
	var workdaysOnly bool
	if err := pool.QueryRow(ctx,
		`INSERT INTO report_settings (user_id) VALUES ($1)
		 RETURNING work_hours_per_day, workdays_only`, userA).
		Scan(&hpd, &workdaysOnly); err != nil {
		t.Fatalf("insert settings com defaults: %v", err)
	}
	if hpd != 8 || !workdaysOnly {
		t.Errorf("defaults esperados 8/true, vieram %g/%v", hpd, workdaysOnly)
	}

	// CHECK barra work_hours_per_day fora de (0,24] (backstop)
	for _, c := range []struct {
		desc string
		sql  string
	}{
		{"hpd = 0", `INSERT INTO report_settings (user_id, work_hours_per_day) VALUES ($1, 0)`},
		{"hpd < 0", `INSERT INTO report_settings (user_id, work_hours_per_day) VALUES ($1, -1)`},
		{"hpd > 24", `INSERT INTO report_settings (user_id, work_hours_per_day) VALUES ($1, 25)`},
	} {
		if _, err := pool.Exec(ctx, c.sql, userB); err == nil {
			t.Errorf("%s deveria violar o CHECK", c.desc)
		}
	}

	// PK em user_id: 1 config por usuário
	if _, err := pool.Exec(ctx,
		`INSERT INTO report_settings (user_id) VALUES ($1)`, userA); err == nil {
		t.Error("2ª linha pro mesmo usuário deveria violar a PK")
	}
}
