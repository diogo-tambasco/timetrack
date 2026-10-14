// availability.go — fase 14: rastreado × não-rastreado (CA-20/21). O
// "esperado" = jornada útil configurável × dias do período (todos, ou só
// seg–sex se workdays_only). Dias contados por calendário no loc (D-06, imune
// a DST). expected_basis registra a suposição (honestidade da fonte).
package reports

import (
	"fmt"
	"math"
)

// Availability é o card rastreado×não-rastreado (contrato §3.1).
type Availability struct {
	TrackedSeconds   int    `json:"tracked_seconds"`
	ExpectedSeconds  int    `json:"expected_seconds"`
	UntrackedSeconds int    `json:"untracked_seconds"`
	TrackedPct       int    `json:"tracked_pct"`
	ExpectedBasis    string `json:"expected_basis"`
}

// availability deriva o card de disponibilidade (D-06): expected = hpd×3600 ×
// nº de dias do período (úteis se workdays_only); untracked = max(exp−tracked,
// 0); tracked_pct round capado em 100, 0 se expected=0 (sem div/0).
func availability(trackedSeconds int, win Window, settings Settings) Availability {
	days := periodDays(win, settings.WorkdaysOnly)
	expected := int(math.Round(settings.WorkHoursPerDay*3600)) * days

	untracked := expected - trackedSeconds
	if untracked < 0 {
		untracked = 0
	}
	pct := 0
	if expected > 0 {
		pct = int(math.Round(float64(trackedSeconds) / float64(expected) * 100))
		if pct > 100 {
			pct = 100
		}
	}

	basis := fmt.Sprintf("%gh/dia × %d dias", settings.WorkHoursPerDay, days)
	if settings.WorkdaysOnly {
		basis = fmt.Sprintf("%gh/dia × %d dias úteis", settings.WorkHoursPerDay, days)
	}

	return Availability{
		TrackedSeconds:   trackedSeconds,
		ExpectedSeconds:  expected,
		UntrackedSeconds: untracked,
		TrackedPct:       pct,
		ExpectedBasis:    basis,
	}
}

// periodDays conta os dias do período no loc (D-06): itera start..end com
// AddDate(+1d), contando seg–sex se workdaysOnly. Aritmética de calendário
// (nunca (end−start)/24h — DST de fuso arbitrário).
func periodDays(win Window, workdaysOnly bool) int {
	n := 0
	for d := win.Start; d.Before(win.End); d = d.AddDate(0, 0, 1) {
		if workdaysOnly {
			wd := d.Weekday()
			if wd == 0 || wd == 6 { // domingo/sábado fora
				continue
			}
		}
		n++
	}
	return n
}
