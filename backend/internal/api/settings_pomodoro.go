// settings_pomodoro.go — GET|PUT /api/v1/settings/pomodoro (T-08, plan
// §3.7). Rota POR USUÁRIO: o user vem da sessão, sem workspace na URL.
// Handlers finos: faixa e upsert moram no timer.Service.
package api

import (
	"net/http"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/reqctx"
	"timetrack.live/backend/internal/timer"
)

func handleGetPomodoro(svc *timer.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		st, err := svc.GetSettings(r.Context(), reqctx.UserID(r.Context()))
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, st)
	}
}

// pomodoroPatch é o corpo do PUT com ponteiros: distingue "ausente" de
// "zero" (CA-02.1). Campo ausente mantém o valor persistido atual (merge),
// pra que um corpo legado curto da fase 03 ({focus,break}) não zere os
// campos novos (ciclos/auto-start/som).
type pomodoroPatch struct {
	FocusMinutes     *int  `json:"focus_minutes"`
	BreakMinutes     *int  `json:"break_minutes"`
	CyclesBeforeLong *int  `json:"cycles_before_long"`
	LongBreakMinutes *int  `json:"long_break_minutes"`
	AutoStartBreak   *bool `json:"auto_start_break"`
	AutoStartFocus   *bool `json:"auto_start_focus"`
	SoundEnabled     *bool `json:"sound_enabled"`
}

func handlePutPomodoro(svc *timer.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var patch pomodoroPatch
		if !decodeJSON(w, r, &patch) {
			return
		}
		userID := reqctx.UserID(r.Context())
		// merge: parte da config atual (ou defaults se nunca salvou) e aplica
		// só os campos presentes no JSON (CA-02.1). SaveSettings valida o
		// objeto inteiro (CA-03).
		cur, err := svc.GetSettings(r.Context(), userID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		if patch.FocusMinutes != nil {
			cur.FocusMinutes = *patch.FocusMinutes
		}
		if patch.BreakMinutes != nil {
			cur.BreakMinutes = *patch.BreakMinutes
		}
		if patch.CyclesBeforeLong != nil {
			cur.CyclesBeforeLong = *patch.CyclesBeforeLong
		}
		if patch.LongBreakMinutes != nil {
			cur.LongBreakMinutes = *patch.LongBreakMinutes
		}
		if patch.AutoStartBreak != nil {
			cur.AutoStartBreak = *patch.AutoStartBreak
		}
		if patch.AutoStartFocus != nil {
			cur.AutoStartFocus = *patch.AutoStartFocus
		}
		if patch.SoundEnabled != nil {
			cur.SoundEnabled = *patch.SoundEnabled
		}
		saved, err := svc.SaveSettings(r.Context(), userID, cur)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, saved)
	}
}
