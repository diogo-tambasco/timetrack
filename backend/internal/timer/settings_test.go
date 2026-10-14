// settings_test.go — fase 15 T-02: timer.Settings estendida (Pomodoro 2.0).
// GetSettings devolve os 7 campos com os defaults certos (CA-01); SaveSettings
// faz upsert ida-e-volta dos 7 (CA-02); valida as faixas novas (CA-03); é
// isolada por usuário (CA-04). Compat: chamada legada (só focus/break) segue
// válida (cycles/long_break normalizados pro default).
package timer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/domain"
	"timetrack.live/backend/internal/timer"
)

func seedUser(t *testing.T, pool *pgxpool.Pool, googleID, email, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (google_id, email, name) VALUES ($1, $2, $3) RETURNING id`,
		googleID, email, name).Scan(&id); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

func newSettingsService(t *testing.T) (*timer.Service, string, string) {
	t.Helper()
	pool := dbtest.NewPool(t)
	userA := seedUser(t, pool, "g-set-a", "seta@tt.dev", "Ana")
	userB := seedUser(t, pool, "g-set-b", "setb@tt.dev", "Bia")
	svc := timer.NewService(pool)
	return svc, userA, userB
}

// CA-01 — sem linha persistida, GetSettings devolve os 7 defaults
// (25/5/4/15/true/true/false). auto-start true/true PRESERVA a fase 03.
func TestGetSettingsDefaultsCompletos(t *testing.T) {
	svc, userA, _ := newSettingsService(t)
	got, err := svc.GetSettings(context.Background(), userA)
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	want := timer.Settings{
		FocusMinutes: 25, BreakMinutes: 5, CyclesBeforeLong: 4,
		LongBreakMinutes: 15, AutoStartBreak: true, AutoStartFocus: true, SoundEnabled: false,
	}
	if got != want {
		t.Errorf("defaults esperados %+v, vieram %+v", want, got)
	}
}

// CA-02 — SaveSettings dos 7 campos persiste e o GetSettings seguinte
// devolve exatamente o que foi salvo (inclusive auto-start OFF e som ON).
func TestSaveSettingsUpsertSeteCampos(t *testing.T) {
	svc, userA, _ := newSettingsService(t)
	ctx := context.Background()
	in := timer.Settings{
		FocusMinutes: 50, BreakMinutes: 10, CyclesBeforeLong: 3,
		LongBreakMinutes: 30, AutoStartBreak: false, AutoStartFocus: false, SoundEnabled: true,
	}
	if _, err := svc.SaveSettings(ctx, userA, in); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	got, err := svc.GetSettings(ctx, userA)
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got != in {
		t.Errorf("GET deveria devolver %+v, veio %+v", in, got)
	}

	// upsert: 2º save sobrescreve
	in2 := timer.Settings{
		FocusMinutes: 25, BreakMinutes: 5, CyclesBeforeLong: 6,
		LongBreakMinutes: 20, AutoStartBreak: true, AutoStartFocus: false, SoundEnabled: false,
	}
	if _, err := svc.SaveSettings(ctx, userA, in2); err != nil {
		t.Fatalf("SaveSettings 2: %v", err)
	}
	got, _ = svc.GetSettings(ctx, userA)
	if got != in2 {
		t.Errorf("upsert deveria devolver %+v, veio %+v", in2, got)
	}
}

// CA-03 — faixas novas (cycles 1–12, long_break 1–120) barram com 400 e
// NADA persiste.
func TestSaveSettingsValidaFaixasNovas(t *testing.T) {
	svc, userA, _ := newSettingsService(t)
	ctx := context.Background()
	base := timer.Settings{
		FocusMinutes: 25, BreakMinutes: 5, CyclesBeforeLong: 4,
		LongBreakMinutes: 15, AutoStartBreak: true, AutoStartFocus: true, SoundEnabled: false,
	}
	if _, err := svc.SaveSettings(ctx, userA, base); err != nil {
		t.Fatalf("save base: %v", err)
	}

	// long_break=0 e cycles=0 são normalizados pro default (compat legada),
	// não falham — testados em TestSaveSettingsLegadoNormaliza. Aqui só os
	// valores que passam do guard mas estouram a faixa.
	bad := []timer.Settings{
		func() timer.Settings { s := base; s.CyclesBeforeLong = 13; return s }(),
		func() timer.Settings { s := base; s.LongBreakMinutes = 121; return s }(),
	}
	for _, b := range bad {
		_, err := svc.SaveSettings(ctx, userA, b)
		var de *domain.Error
		if !errors.As(err, &de) || de.Code != "validation_error" {
			// o guard do Go (não o CHECK do banco) DEVE pegar antes do SQL
			t.Errorf("SaveSettings %+v deveria falhar com validation_error, veio %v", b, err)
		}
	}
	// cycles=0 é normalizado pro default (compat legada), NÃO falha
	cyclesZero := base
	cyclesZero.CyclesBeforeLong = 0
	if _, err := svc.SaveSettings(ctx, userA, cyclesZero); err != nil {
		t.Errorf("cycles=0 deveria normalizar pro default e salvar, veio erro: %v", err)
	}

	// nada dos inválidos persistiu: segue o base/normalizado (cycles default 4)
	got, _ := svc.GetSettings(ctx, userA)
	if got.CyclesBeforeLong != 4 || got.LongBreakMinutes != 15 {
		t.Errorf("config deveria seguir cycles=4 long_break=15 após inválidos, veio %d/%d",
			got.CyclesBeforeLong, got.LongBreakMinutes)
	}
}

// Compat (CA-02.1 no nível do serviço) — chamada legada só com focus/break
// (cycles/long_break zero) é normalizada e salva sem erro.
func TestSaveSettingsLegadoNormaliza(t *testing.T) {
	svc, userA, _ := newSettingsService(t)
	ctx := context.Background()
	if _, err := svc.SaveSettings(ctx, userA, timer.Settings{FocusMinutes: 25, BreakMinutes: 5}); err != nil {
		t.Fatalf("chamada legada deveria salvar: %v", err)
	}
	got, _ := svc.GetSettings(ctx, userA)
	if got.CyclesBeforeLong != 4 || got.LongBreakMinutes != 15 {
		t.Errorf("legado deveria normalizar cycles/long_break pros defaults 4/15, veio %d/%d",
			got.CyclesBeforeLong, got.LongBreakMinutes)
	}
}

// CA-04 — leitura/escrita usam exclusivamente o user_id da sessão; a linha
// de outro usuário NUNCA é lida nem sobrescrita.
func TestSettingsIsolamentoPorUsuario(t *testing.T) {
	svc, userA, userB := newSettingsService(t)
	ctx := context.Background()
	inA := timer.Settings{
		FocusMinutes: 40, BreakMinutes: 8, CyclesBeforeLong: 2,
		LongBreakMinutes: 25, AutoStartBreak: false, AutoStartFocus: true, SoundEnabled: true,
	}
	if _, err := svc.SaveSettings(ctx, userA, inA); err != nil {
		t.Fatalf("save A: %v", err)
	}
	// B nunca salvou: vê os defaults, não os de A
	gotB, _ := svc.GetSettings(ctx, userB)
	if gotB != (timer.Settings{
		FocusMinutes: 25, BreakMinutes: 5, CyclesBeforeLong: 4,
		LongBreakMinutes: 15, AutoStartBreak: true, AutoStartFocus: true, SoundEnabled: false,
	}) {
		t.Errorf("B deveria ver os defaults, viu %+v", gotB)
	}
	// salvar pra B não toca em A
	if _, err := svc.SaveSettings(ctx, userB, timer.Settings{
		FocusMinutes: 60, BreakMinutes: 20, CyclesBeforeLong: 5,
		LongBreakMinutes: 40, AutoStartBreak: true, AutoStartFocus: false, SoundEnabled: false,
	}); err != nil {
		t.Fatalf("save B: %v", err)
	}
	gotA, _ := svc.GetSettings(ctx, userA)
	if gotA != inA {
		t.Errorf("config de A deveria seguir intacta após save de B, veio %+v", gotA)
	}
}
