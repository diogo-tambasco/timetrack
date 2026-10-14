package calendar_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/calendar"
	"timetrack.live/backend/internal/db/dbtest"
)

// svcForSettings monta um Service só pra exercitar GetSettings/SaveSettings
// (oauth/key dummy — settings só usa o pool).
func svcForSettings(t *testing.T) (*calendar.Service, *pgxpool.Pool) {
	t.Helper()
	pool := dbtest.NewPool(t)
	return calendar.NewService(pool, oauthCfg(""), testKey(), ""), pool
}

func anyExpiry() time.Time { return time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC) }

// CA-01 — sem linha persistida, GetSettings devolve os defaults.
func TestGetSettingsDefaults(t *testing.T) {
	svc, pool := svcForSettings(t)
	uid := seedUser(t, pool, testKey(), "g-s1", "tok", "ref", []string{}, anyExpiry())

	got, err := svc.GetSettings(context.Background(), uid)
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	want := calendar.Settings{DefaultView: "week", HourStart: 6, HourEnd: 23, Density: "comfortable"}
	if got != want {
		t.Errorf("defaults esperados %+v, vieram %+v", want, got)
	}
}

// CA-02 — SaveSettings válido faz upsert e o GET seguinte devolve o salvo.
func TestSaveSettingsUpsert(t *testing.T) {
	svc, pool := svcForSettings(t)
	uid := seedUser(t, pool, testKey(), "g-s2", "tok", "ref", []string{}, anyExpiry())

	in := calendar.Settings{DefaultView: "day", HourStart: 8, HourEnd: 20, Density: "compact"}
	saved, err := svc.SaveSettings(context.Background(), uid, in)
	if err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if saved != in {
		t.Errorf("Save deveria ecoar %+v, veio %+v", in, saved)
	}
	got, err := svc.GetSettings(context.Background(), uid)
	if err != nil || got != in {
		t.Errorf("GET deveria devolver %+v, veio %+v (err=%v)", in, got, err)
	}

	// upsert: segundo Save atualiza a MESMA linha (não duplica)
	in2 := calendar.Settings{DefaultView: "month", HourStart: 6, HourEnd: 23, Density: "comfortable"}
	if _, err := svc.SaveSettings(context.Background(), uid, in2); err != nil {
		t.Fatalf("SaveSettings 2: %v", err)
	}
	if got, _ := svc.GetSettings(context.Background(), uid); got != in2 {
		t.Errorf("após 2º save esperava %+v, veio %+v", in2, got)
	}
}

// CA-03 — cada limite de validação rejeita (domain Validation) sem persistir.
func TestSaveSettingsValidacao(t *testing.T) {
	svc, pool := svcForSettings(t)
	uid := seedUser(t, pool, testKey(), "g-s3", "tok", "ref", []string{}, anyExpiry())

	base := calendar.Settings{DefaultView: "week", HourStart: 6, HourEnd: 23, Density: "comfortable"}
	for _, c := range []struct {
		desc string
		in   calendar.Settings
	}{
		{"view inválida", calendar.Settings{DefaultView: "year", HourStart: 6, HourEnd: 23, Density: "comfortable"}},
		{"density inválida", calendar.Settings{DefaultView: "week", HourStart: 6, HourEnd: 23, Density: "cozy"}},
		{"hour_start < 0", calendar.Settings{DefaultView: "week", HourStart: -1, HourEnd: 23, Density: "comfortable"}},
		{"hour_start > 23", calendar.Settings{DefaultView: "week", HourStart: 24, HourEnd: 24, Density: "comfortable"}},
		{"hour_end < 1", calendar.Settings{DefaultView: "week", HourStart: 0, HourEnd: 0, Density: "comfortable"}},
		{"hour_end > 24", calendar.Settings{DefaultView: "week", HourStart: 6, HourEnd: 25, Density: "comfortable"}},
		{"hour_end <= hour_start", calendar.Settings{DefaultView: "week", HourStart: 10, HourEnd: 10, Density: "comfortable"}},
	} {
		_, err := svc.SaveSettings(context.Background(), uid, c.in)
		assertDomain(t, err, 400, "validation_error")
		_ = base
	}

	// nada persistiu: GET segue nos defaults
	if got, _ := svc.GetSettings(context.Background(), uid); got != base {
		t.Errorf("nenhum inválido deveria persistir; GET veio %+v", got)
	}
}

// CA-04 — leitura/escrita usam SÓ o user_id da sessão: a linha de A nunca é
// lida por B (mutação removendo WHERE user_id morre aqui).
func TestSettingsIsolamentoPorUsuario(t *testing.T) {
	svc, pool := svcForSettings(t)
	a := seedUser(t, pool, testKey(), "g-iso-a", "tok", "ref", []string{}, anyExpiry())
	b := seedUser(t, pool, testKey(), "g-iso-b", "tok", "ref", []string{}, anyExpiry())

	inA := calendar.Settings{DefaultView: "day", HourStart: 7, HourEnd: 19, Density: "compact"}
	if _, err := svc.SaveSettings(context.Background(), a, inA); err != nil {
		t.Fatalf("save A: %v", err)
	}
	// B nunca salvou → defaults, NÃO os de A
	gotB, _ := svc.GetSettings(context.Background(), b)
	want := calendar.Settings{DefaultView: "week", HourStart: 6, HourEnd: 23, Density: "comfortable"}
	if gotB != want {
		t.Errorf("B deveria ler defaults, veio %+v (vazou de A?)", gotB)
	}
	// A segue com o seu
	if gotA, _ := svc.GetSettings(context.Background(), a); gotA != inA {
		t.Errorf("A deveria ler %+v, veio %+v", inA, gotA)
	}
}
