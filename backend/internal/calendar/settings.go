// settings.go — fase 11 (plan §3): preferências da UI de calendário POR
// USUÁRIO (sincroniza entre dispositivos). Espelho FIEL do timer/settings.go
// (pomodoro): defaults na leitura sem linha, validação ANTES do SQL, upsert
// por user_id. Sem linha = defaults (CA-01); os CHECKs da migração 0012 são
// o backstop (CA-03).
package calendar

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"timetrack.live/backend/internal/domain"
)

// Settings é o contrato do GET|PUT /api/v1/settings/calendar (plan §3).
type Settings struct {
	DefaultView string `json:"default_view"`
	HourStart   int    `json:"hour_start"`
	HourEnd     int    `json:"hour_end"`
	Density     string `json:"density"`
}

// defaultSettings — aplicados na leitura quando o usuário nunca salvou
// (CA-01). Idênticos aos DEFAULTs da migração 0012.
func defaultSettings() Settings {
	return Settings{DefaultView: "week", HourStart: 6, HourEnd: 23, Density: "comfortable"}
}

// GetSettings devolve a config persistida do usuário; sem linha valem os
// defaults 6/23/week/comfortable (CA-01). Lê exclusivamente pelo user_id da
// sessão (CA-04).
func (s *Service) GetSettings(ctx context.Context, userID string) (Settings, error) {
	var out Settings
	err := s.db.QueryRow(ctx, `
		SELECT default_view, hour_start, hour_end, density
		  FROM calendar_settings
		 WHERE user_id = $1`, userID).
		Scan(&out.DefaultView, &out.HourStart, &out.HourEnd, &out.Density)
	if errors.Is(err, pgx.ErrNoRows) {
		return defaultSettings(), nil
	}
	if err != nil {
		return Settings{}, err
	}
	return out, nil
}

// SaveSettings valida (enum/faixa) ANTES do SQL e faz upsert por usuário
// (CA-02/03/04); inválido não persiste nada. Os CHECKs do banco (migração
// 0012) são o backstop.
func (s *Service) SaveSettings(ctx context.Context, userID string, in Settings) (Settings, error) {
	if in.DefaultView != "day" && in.DefaultView != "week" && in.DefaultView != "month" {
		return Settings{}, domain.Validation("default_view deve ser day, week ou month")
	}
	if in.Density != "comfortable" && in.Density != "compact" {
		return Settings{}, domain.Validation("density deve ser comfortable ou compact")
	}
	if in.HourStart < 0 || in.HourStart > 23 {
		return Settings{}, domain.Validation("hour_start deve estar entre 0 e 23")
	}
	if in.HourEnd < 1 || in.HourEnd > 24 {
		return Settings{}, domain.Validation("hour_end deve estar entre 1 e 24")
	}
	if in.HourEnd <= in.HourStart {
		return Settings{}, domain.Validation("hour_end deve ser maior que hour_start")
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO calendar_settings (user_id, default_view, hour_start, hour_end, density)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE
		   SET default_view = excluded.default_view,
		       hour_start   = excluded.hour_start,
		       hour_end     = excluded.hour_end,
		       density      = excluded.density,
		       updated_at   = now()`,
		userID, in.DefaultView, in.HourStart, in.HourEnd, in.Density)
	if err != nil {
		return Settings{}, err
	}
	return in, nil
}
