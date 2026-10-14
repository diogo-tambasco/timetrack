package config

import (
	"strings"
	"testing"
	"time"
)

// CA-06: DATABASE_URL ausente → erro claro citando a variável.
func TestConfig_SemDatabaseURL_Erra(t *testing.T) {
	getenv := func(key string) string { return "" }

	_, err := Load(getenv)
	if err == nil {
		t.Fatal("esperava erro com DATABASE_URL ausente, veio nil")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("erro deve citar DATABASE_URL, veio: %v", err)
	}
}

func TestConfig_Completa_Carrega(t *testing.T) {
	env := map[string]string{
		"DATABASE_URL": "postgres://u:p@localhost:5432/tt",
		"ADDR":         ":9090",
		"STATIC_DIR":   "/tmp/dist",
	}
	getenv := func(key string) string { return env[key] }

	cfg, err := Load(getenv)
	if err != nil {
		t.Fatalf("esperava carregar sem erro, veio: %v", err)
	}
	if cfg.DatabaseURL != "postgres://u:p@localhost:5432/tt" {
		t.Errorf("DatabaseURL errada: %q", cfg.DatabaseURL)
	}
	if cfg.Addr != ":9090" {
		t.Errorf("Addr errado: %q", cfg.Addr)
	}
	if cfg.StaticDir != "/tmp/dist" {
		t.Errorf("StaticDir errado: %q", cfg.StaticDir)
	}
}

func TestConfig_AddrDefault(t *testing.T) {
	env := map[string]string{"DATABASE_URL": "postgres://u:p@localhost:5432/tt"}
	getenv := func(key string) string { return env[key] }

	cfg, err := Load(getenv)
	if err != nil {
		t.Fatalf("esperava carregar sem erro, veio: %v", err)
	}
	if cfg.Addr != ":8080" {
		t.Errorf("Addr default deve ser :8080, veio %q", cfg.Addr)
	}
}

// Fase 16 — VAPID + GUARD_TICK. Ausentes não abortam o boot (RNF-04): push
// desligado (chaves vazias) e tick zero (worker aplica o default de 60s).
func TestConfig_VapidEGuardTick_Ausentes(t *testing.T) {
	env := map[string]string{"DATABASE_URL": "postgres://u:p@localhost:5432/tt"}
	cfg, err := Load(func(k string) string { return env[k] })
	if err != nil {
		t.Fatalf("ausência de VAPID/GUARD_TICK NÃO pode abortar o boot: %v", err)
	}
	if cfg.VapidPublicKey != "" || cfg.VapidPrivateKey != "" || cfg.VapidSubject != "" {
		t.Errorf("sem env, as chaves VAPID deveriam ser vazias (push desligado)")
	}
	if cfg.GuardTick != 0 {
		t.Errorf("sem GUARD_TICK, o tick deveria ser 0 (worker aplica 60s), veio %v", cfg.GuardTick)
	}
}

// VAPID + GUARD_TICK presentes são lidos; GUARD_TICK parseado como duração.
func TestConfig_VapidEGuardTick_Presentes(t *testing.T) {
	env := map[string]string{
		"DATABASE_URL":      "postgres://u:p@localhost:5432/tt",
		"VAPID_PUBLIC_KEY":  "pub",
		"VAPID_PRIVATE_KEY": "priv",
		"VAPID_SUBJECT":     "mailto:x@y.z",
		"GUARD_TICK":        "5s",
	}
	cfg, err := Load(func(k string) string { return env[k] })
	if err != nil {
		t.Fatalf("carregar: %v", err)
	}
	if cfg.VapidPublicKey != "pub" || cfg.VapidPrivateKey != "priv" || cfg.VapidSubject != "mailto:x@y.z" {
		t.Errorf("chaves VAPID não carregadas: %+v", cfg)
	}
	if cfg.GuardTick != 5_000_000_000 { // 5s em ns
		t.Errorf("GUARD_TICK deveria parsear 5s, veio %v", cfg.GuardTick)
	}
}

// Fase 20 — REMINDER_TICK espelha o GUARD_TICK: ausente/inválido → 0 (worker
// aplica 60s); presente → parseado como duração. Não aborta o boot.
func TestConfig_ReminderTick(t *testing.T) {
	// ausente → 0
	env := map[string]string{"DATABASE_URL": "postgres://u:p@localhost:5432/tt"}
	cfg, err := Load(func(k string) string { return env[k] })
	if err != nil {
		t.Fatalf("ausência de REMINDER_TICK NÃO pode abortar o boot: %v", err)
	}
	if cfg.ReminderTick != 0 {
		t.Errorf("sem REMINDER_TICK, o tick deveria ser 0 (worker aplica 60s), veio %v", cfg.ReminderTick)
	}
	// presente → parseado
	env["REMINDER_TICK"] = "30s"
	cfg, err = Load(func(k string) string { return env[k] })
	if err != nil {
		t.Fatalf("carregar: %v", err)
	}
	if cfg.ReminderTick != 30_000_000_000 { // 30s em ns
		t.Errorf("REMINDER_TICK deveria parsear 30s, veio %v", cfg.ReminderTick)
	}
	// inválido → 0
	env["REMINDER_TICK"] = "lixo"
	cfg, _ = Load(func(k string) string { return env[k] })
	if cfg.ReminderTick != 0 {
		t.Errorf("REMINDER_TICK inválido deveria cair em 0, veio %v", cfg.ReminderTick)
	}
}

// GUARD_TICK inválido cai em 0 (sem abortar — worker usa o default).
func TestConfig_GuardTickInvalido(t *testing.T) {
	env := map[string]string{
		"DATABASE_URL": "postgres://u:p@localhost:5432/tt",
		"GUARD_TICK":   "não-é-duração",
	}
	cfg, err := Load(func(k string) string { return env[k] })
	if err != nil {
		t.Fatalf("GUARD_TICK inválido NÃO pode abortar o boot: %v", err)
	}
	if cfg.GuardTick != 0 {
		t.Errorf("GUARD_TICK inválido deveria virar 0, veio %v", cfg.GuardTick)
	}
}

// Fase 17 — INVITE_TTL ausente/inválido → default 14 dias (336h); válido é
// parseado. Nunca aborta o boot.
func TestConfig_InviteTTL(t *testing.T) {
	defaultTTL := 336 * time.Hour

	// ausente → default.
	cfg, err := Load(func(k string) string {
		return map[string]string{"DATABASE_URL": "postgres://u:p@localhost:5432/tt"}[k]
	})
	if err != nil {
		t.Fatalf("ausência de INVITE_TTL NÃO pode abortar o boot: %v", err)
	}
	if cfg.InviteTTL != defaultTTL {
		t.Errorf("sem INVITE_TTL, esperava o default 336h, veio %v", cfg.InviteTTL)
	}

	// inválido → default.
	cfg, err = Load(func(k string) string {
		return map[string]string{
			"DATABASE_URL": "postgres://u:p@localhost:5432/tt",
			"INVITE_TTL":   "xpto",
		}[k]
	})
	if err != nil {
		t.Fatalf("INVITE_TTL inválido NÃO pode abortar o boot: %v", err)
	}
	if cfg.InviteTTL != defaultTTL {
		t.Errorf("INVITE_TTL inválido deveria cair no default 336h, veio %v", cfg.InviteTTL)
	}

	// válido → parseado.
	cfg, err = Load(func(k string) string {
		return map[string]string{
			"DATABASE_URL": "postgres://u:p@localhost:5432/tt",
			"INVITE_TTL":   "48h",
		}[k]
	})
	if err != nil {
		t.Fatalf("carregar: %v", err)
	}
	if cfg.InviteTTL != 48*time.Hour {
		t.Errorf("INVITE_TTL=48h deveria parsear, veio %v", cfg.InviteTTL)
	}
}
