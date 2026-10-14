// settings.go — config de Pomodoro por usuário (T-08 fase 03, estendida na
// fase 15). É hábito da pessoa, não do workspace (D-08): vale em qualquer
// device. A fase 15 adiciona ciclos/pausa-longa/auto-start/som (aditivo —
// RNF-07); os defaults preservam o comportamento da fase 03 (D-05).
package timer

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"timetrack.live/backend/internal/domain"
	"timetrack.live/backend/internal/entries"
)

// Settings é o contrato do GET|PUT /api/v1/settings/pomodoro (plan §3.7 /
// fase 15 §3.1). Campos novos da fase 15 são ADITIVOS — nenhum campo
// existente muda de tipo/semântica (RNF-07).
type Settings struct {
	FocusMinutes     int  `json:"focus_minutes"`
	BreakMinutes     int  `json:"break_minutes"`
	CyclesBeforeLong int  `json:"cycles_before_long"`
	LongBreakMinutes int  `json:"long_break_minutes"`
	AutoStartBreak   bool `json:"auto_start_break"`
	AutoStartFocus   bool `json:"auto_start_focus"`
	SoundEnabled     bool `json:"sound_enabled"`
}

// defaultSettings espelha os defaults da migração 0010+0015 (CA-01/D-05):
// auto-start true/true PRESERVA o ciclo automático foco↔pausa da fase 03.
func defaultSettings() Settings {
	return Settings{
		FocusMinutes:     25,
		BreakMinutes:     5,
		CyclesBeforeLong: 4,
		LongBreakMinutes: 15,
		AutoStartBreak:   true,
		AutoStartFocus:   true,
		SoundEnabled:     false,
	}
}

// GetSettings devolve a config persistida; sem linha valem os defaults da
// migração (CA-01 — 25/5/4/15/true/true/false).
func (s *Service) GetSettings(ctx context.Context, userID string) (Settings, error) {
	return loadFullSettings(ctx, s.db, userID)
}

// loadFullSettings lê os 7 campos da config do usuário; sem linha salva
// valem os defaults (CA-01). Usado por GetSettings e por Start/Pause.
func loadFullSettings(ctx context.Context, q entries.Querier, userID string) (Settings, error) {
	var st Settings
	err := q.QueryRow(ctx, `
		SELECT focus_minutes, break_minutes, cycles_before_long,
		       long_break_minutes, auto_start_break, auto_start_focus, sound_enabled
		  FROM pomodoro_settings WHERE user_id = $1`, userID).
		Scan(&st.FocusMinutes, &st.BreakMinutes, &st.CyclesBeforeLong,
			&st.LongBreakMinutes, &st.AutoStartBreak, &st.AutoStartFocus, &st.SoundEnabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return defaultSettings(), nil
	}
	if err != nil {
		return Settings{}, err
	}
	return st, nil
}

// SaveSettings valida as faixas (focus 15–90, break 1–60, long_break 1–120,
// cycles 1–12) ANTES do SQL e faz upsert por usuário; fora da faixa nada
// persiste (CA-03). Os CHECKs do banco (migrações 0010/0015) são o backstop.
//
// Compat com chamadas legadas (fase 03): campos numéricos novos com valor
// ZERO são normalizados pros defaults antes de validar — assim
// `Settings{FocusMinutes:25, BreakMinutes:5}` (sem ciclos) segue válido.
func (s *Service) SaveSettings(ctx context.Context, userID string, in Settings) (Settings, error) {
	if in.CyclesBeforeLong == 0 {
		in.CyclesBeforeLong = defaultSettings().CyclesBeforeLong
	}
	if in.LongBreakMinutes == 0 {
		in.LongBreakMinutes = defaultSettings().LongBreakMinutes
	}
	if in.FocusMinutes < 15 || in.FocusMinutes > 90 {
		return Settings{}, domain.Validation("focus_minutes deve estar entre 15 e 90")
	}
	if in.BreakMinutes < 1 || in.BreakMinutes > 60 {
		return Settings{}, domain.Validation("break_minutes deve estar entre 1 e 60")
	}
	if in.LongBreakMinutes < 1 || in.LongBreakMinutes > 120 {
		return Settings{}, domain.Validation("long_break_minutes deve estar entre 1 e 120")
	}
	if in.CyclesBeforeLong < 1 || in.CyclesBeforeLong > 12 {
		return Settings{}, domain.Validation("cycles_before_long deve estar entre 1 e 12")
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO pomodoro_settings (user_id, focus_minutes, break_minutes,
		            cycles_before_long, long_break_minutes,
		            auto_start_break, auto_start_focus, sound_enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id) DO UPDATE
		   SET focus_minutes = excluded.focus_minutes,
		       break_minutes = excluded.break_minutes,
		       cycles_before_long = excluded.cycles_before_long,
		       long_break_minutes = excluded.long_break_minutes,
		       auto_start_break = excluded.auto_start_break,
		       auto_start_focus = excluded.auto_start_focus,
		       sound_enabled = excluded.sound_enabled,
		       updated_at = now()`,
		userID, in.FocusMinutes, in.BreakMinutes, in.CyclesBeforeLong,
		in.LongBreakMinutes, in.AutoStartBreak, in.AutoStartFocus, in.SoundEnabled)
	if err != nil {
		return Settings{}, err
	}
	return in, nil
}
