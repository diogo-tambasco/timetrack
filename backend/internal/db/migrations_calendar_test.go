package db

import (
	"context"
	"testing"
)

// Fase 11 T-01 — TestMigracaoCalendarSettings: preferências da UI de
// calendário POR USUÁRIO (plan §2). Defaults 6/23/week/comfortable na
// linha; CHECKs barram view/density/hora inválidas e hour_end<=hour_start
// (CA-01 backstop / CA-03). PK em user_id (1 config por usuário).
func TestMigracaoCalendarSettings(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool, Migrations()); err != nil {
		t.Fatalf("migrações reais falharam: %v", err)
	}

	var ok bool
	if err := pool.QueryRow(ctx,
		"SELECT to_regclass('calendar_settings') IS NOT NULL").Scan(&ok); err != nil || !ok {
		t.Fatalf("tabela calendar_settings não existe (err=%v)", err)
	}

	var userA, userB string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-a', 'a@b.c', 'Ana') RETURNING id`).Scan(&userA); err != nil {
		t.Fatalf("insert user A: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-b', 'b@b.c', 'Bia') RETURNING id`).Scan(&userB); err != nil {
		t.Fatalf("insert user B: %v", err)
	}

	// insert sem valores aplica os defaults 6/23/week/comfortable (CA-01)
	var view, density string
	var hStart, hEnd int
	if err := pool.QueryRow(ctx,
		`INSERT INTO calendar_settings (user_id) VALUES ($1)
		 RETURNING default_view, hour_start, hour_end, density`, userA).
		Scan(&view, &hStart, &hEnd, &density); err != nil {
		t.Fatalf("insert settings com defaults: %v", err)
	}
	if view != "week" || hStart != 6 || hEnd != 23 || density != "comfortable" {
		t.Errorf("defaults esperados week/6/23/comfortable, vieram %s/%d/%d/%s", view, hStart, hEnd, density)
	}

	// CHECKs barram valores inválidos (CA-03 no banco — backstop)
	for _, c := range []struct {
		desc string
		sql  string
	}{
		{"default_view inválido", `INSERT INTO calendar_settings (user_id, default_view) VALUES ($1, 'year')`},
		{"density inválida", `INSERT INTO calendar_settings (user_id, density) VALUES ($1, 'cozy')`},
		{"hour_start < 0", `INSERT INTO calendar_settings (user_id, hour_start) VALUES ($1, -1)`},
		{"hour_start > 23", `INSERT INTO calendar_settings (user_id, hour_start) VALUES ($1, 24)`},
		{"hour_end < 1", `INSERT INTO calendar_settings (user_id, hour_end) VALUES ($1, 0)`},
		{"hour_end > 24", `INSERT INTO calendar_settings (user_id, hour_end) VALUES ($1, 25)`},
		{"hour_end <= hour_start", `INSERT INTO calendar_settings (user_id, hour_start, hour_end) VALUES ($1, 10, 10)`},
	} {
		if _, err := pool.Exec(ctx, c.sql, userB); err == nil {
			t.Errorf("%s deveria violar um CHECK", c.desc)
		}
	}

	// PK em user_id: 1 config por usuário
	if _, err := pool.Exec(ctx,
		`INSERT INTO calendar_settings (user_id) VALUES ($1)`, userA); err == nil {
		t.Error("2ª linha pro mesmo usuário deveria violar a PK")
	}
}
