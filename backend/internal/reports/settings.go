// settings.go — fase 14: preferências do relatório POR USUÁRIO (jornada útil
// esperada — base do rastreado×não-rastreado, CA-21/29). Espelho FIEL do
// calendar/settings.go (fase 11): defaults na leitura sem linha, validação
// ANTES do SQL, upsert por user_id. Sem linha = defaults (CA-29). Os CHECKs da
// migração 0014 são o backstop. (Get/Save ganham testes na T-05.)
package reports

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"timetrack.live/backend/internal/domain"
)

// Settings é o contrato do GET|PUT /api/v1/settings/reports (plan §3.3).
type Settings struct {
	WorkHoursPerDay float64 `json:"work_hours_per_day"`
	WorkdaysOnly    bool    `json:"workdays_only"`
}

// DefaultSettings — jornada útil de 8h, só dias úteis (CA-21). Idênticos aos
// DEFAULTs da migração 0014.
func DefaultSettings() Settings {
	return Settings{WorkHoursPerDay: 8, WorkdaysOnly: true}
}

// GetSettings devolve a config persistida; sem linha → defaults (CA-29). Lê só
// pelo user_id da sessão.
func (s *Service) GetSettings(ctx context.Context, userID string) (Settings, error) {
	var out Settings
	err := s.db.QueryRow(ctx, `
		SELECT work_hours_per_day, workdays_only
		  FROM report_settings
		 WHERE user_id = $1`, userID).
		Scan(&out.WorkHoursPerDay, &out.WorkdaysOnly)
	if errors.Is(err, pgx.ErrNoRows) {
		return DefaultSettings(), nil
	}
	if err != nil {
		return Settings{}, err
	}
	return out, nil
}

// SaveSettings valida (0<hpd≤24) ANTES do SQL e faz upsert por usuário (CA-29);
// inválido não persiste. Os CHECKs do banco (migração 0014) são o backstop.
func (s *Service) SaveSettings(ctx context.Context, userID string, in Settings) (Settings, error) {
	if in.WorkHoursPerDay <= 0 || in.WorkHoursPerDay > 24 {
		return Settings{}, domain.Validation("work_hours_per_day deve estar entre 0 (exclusivo) e 24")
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO report_settings (user_id, work_hours_per_day, workdays_only)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE
		   SET work_hours_per_day = excluded.work_hours_per_day,
		       workdays_only      = excluded.workdays_only,
		       updated_at         = now()`,
		userID, in.WorkHoursPerDay, in.WorkdaysOnly)
	if err != nil {
		return Settings{}, err
	}
	return in, nil
}
