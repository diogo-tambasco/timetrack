// Package config carrega a configuração do servidor a partir de variáveis
// de ambiente (RF-03 da fase 00). Toda configuração é 100% por env.
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

type Config struct {
	DatabaseURL string // DSN do Postgres (obrigatória)
	Addr        string // listener HTTP (default :8080)
	StaticDir   string // override do diretório de estáticos em dev (vazio = embed)

	// Fase 01 — OAuth Google + sessão (plan §3.6). Placeholders bastam
	// pro build/testes (Google stubbado); o login REAL exige a credencial
	// do Google Cloud Console (PENDENCIAS-DIOGO P-01).
	GoogleClientID     string
	GoogleClientSecret string
	OAuthRedirectURL   string
	CookieSecure       bool
	TokenKey           []byte // 32 bytes — AES-256-GCM (TT_TOKEN_KEY, hex)

	// Overrides dos endpoints do Google pra teste/E2E (vazios → Google real).
	GoogleAuthURL     string
	GoogleTokenURL    string
	GoogleUserinfoURL string

	// Fase 06: base da Google Calendar API (vazio → API real). E2E injeta o
	// calendarstub.
	GoogleCalendarAPIURL string

	// Fase 16 — anti-timer-esquecido. As chaves VAPID ficam no env (config de
	// deploy, NÃO dependência de terceiro — RNF-04): a PRIVATE nunca sai do
	// servidor. Sem VapidPublicKey → push desligado (o resto funciona). O boot
	// NÃO exige nenhuma destas (diferente do DATABASE_URL).
	VapidPublicKey  string
	VapidPrivateKey string
	VapidSubject    string        // "mailto:..." exigido pelo protocolo VAPID
	GuardTick       time.Duration // intervalo do worker de corte (default 60s)

	// Fase 20 — intervalo do worker de lembretes (REMINDER_TICK, duration).
	// Ausente/inválido/<=0 → 0 (o reminders.NewWorker aplica o default de 60s).
	// NÃO obrigatório pro boot. Espelha o GuardTick.
	ReminderTick time.Duration

	// Fase 17 — TTL do convite pending (INVITE_TTL, duration). Ausente/inválido
	// → default 14 dias (336h). NÃO obrigatório pro boot.
	InviteTTL time.Duration
}

// Load lê a configuração via getenv (injetável pra teste).
func Load(getenv func(string) string) (Config, error) {
	dsn := getenv("DATABASE_URL")
	if dsn == "" {
		return Config{}, fmt.Errorf("variável de ambiente DATABASE_URL é obrigatória e não está definida")
	}

	addr := getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	tokenKey, err := loadTokenKey(getenv("TT_TOKEN_KEY"))
	if err != nil {
		return Config{}, err
	}

	clientID := getenv("GOOGLE_CLIENT_ID")
	if clientID == "" {
		clientID = "dev-client-id-placeholder"
	}
	clientSecret := getenv("GOOGLE_CLIENT_SECRET")
	if clientSecret == "" {
		clientSecret = "dev-client-secret-placeholder"
	}
	redirectURL := getenv("OAUTH_REDIRECT_URL")
	if redirectURL == "" {
		redirectURL = "http://localhost:8080/auth/google/callback"
	}

	return Config{
		DatabaseURL: dsn,
		Addr:        addr,
		StaticDir:   getenv("STATIC_DIR"),

		GoogleClientID:     clientID,
		GoogleClientSecret: clientSecret,
		OAuthRedirectURL:   redirectURL,
		CookieSecure:       getenv("COOKIE_SECURE") == "true",
		TokenKey:           tokenKey,

		GoogleAuthURL:     getenv("GOOGLE_AUTH_URL"),
		GoogleTokenURL:    getenv("GOOGLE_TOKEN_URL"),
		GoogleUserinfoURL: getenv("GOOGLE_USERINFO_URL"),

		GoogleCalendarAPIURL: getenv("GOOGLE_CALENDAR_API_URL"),

		VapidPublicKey:  getenv("VAPID_PUBLIC_KEY"),
		VapidPrivateKey: getenv("VAPID_PRIVATE_KEY"),
		VapidSubject:    getenv("VAPID_SUBJECT"),
		GuardTick:       loadGuardTick(getenv("GUARD_TICK")),
		ReminderTick:    loadGuardTick(getenv("REMINDER_TICK")), // mesma semântica do GuardTick
		InviteTTL:       loadInviteTTL(getenv("INVITE_TTL")),
	}, nil
}

// loadInviteTTL parseia INVITE_TTL (ex.: "336h", "14d" não vale — só formatos
// de time.ParseDuration). Vazio/inválido/<=0 → default 14 dias (336h). Nunca
// aborta o boot.
func loadInviteTTL(raw string) time.Duration {
	const def = 336 * time.Hour
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

// loadGuardTick parseia GUARD_TICK (ex.: "60s", "5m"). Vazio/inválido/<=0 → 0
// (o guard.NewWorker aplica o default de 60s). Nunca aborta o boot (RNF-04).
func loadGuardTick(raw string) time.Duration {
	if raw == "" {
		return 0
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0
	}
	return d
}

// loadTokenKey decodifica TT_TOKEN_KEY (64 chars hex = 32 bytes). Vazia →
// chave de DEV determinística (insegura de propósito; prod define a real).
func loadTokenKey(raw string) ([]byte, error) {
	if raw == "" {
		devKey := sha256.Sum256([]byte("timetrack-dev-insecure-token-key"))
		return devKey[:], nil
	}
	key, err := hex.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("TT_TOKEN_KEY deve ter 64 caracteres hex (32 bytes), veio %d chars (err=%v)", len(raw), err)
	}
	return key, nil
}
