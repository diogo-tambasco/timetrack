// kpis.go — fase 14: foco por hora (CA-22/23) e a faixa de KPIs (CA-24).
// Tudo deriva da `list` já carregada (D-08) — zero query nova. 24 buckets
// sempre (zeros), bucket pela hora local do started_at (v1). KPIs com div/0
// protegida (nunca NaN).
package reports

import (
	"math"
	"time"
)

// FocusHourBucket é um bucket de foco por hora local (CA-22/23).
type FocusHourBucket struct {
	Hour         int `json:"hour"`
	FocusSeconds int `json:"focus_seconds"`
}

// Kpis é a faixa de KPIs (CA-24) — tudo derivado das seções já calculadas.
type Kpis struct {
	TrackedSeconds    int `json:"tracked_seconds"`
	DeepFocusPct      int `json:"deep_focus_pct"`
	AvgSessionSeconds int `json:"avg_session_seconds"`
	UntrackedSeconds  int `json:"untracked_seconds"`
}

// focusByHour soma focus_seconds (entries ≥1500s) no bucket da hora local do
// started_at (D-08). Sempre 24 buckets (0–23), zeros inclusos (CA-23).
func focusByHour(list []entryRow, loc *time.Location) []FocusHourBucket {
	sums := [24]int{}
	for _, e := range list {
		if e.Duration < focusThresholdSeconds {
			continue
		}
		h := e.StartedAt.In(loc).Hour()
		sums[h] += e.Duration
	}
	out := make([]FocusHourBucket, 24)
	for h := 0; h < 24; h++ {
		out[h] = FocusHourBucket{Hour: h, FocusSeconds: sums[h]}
	}
	return out
}

// kpis monta a faixa (CA-24): deep_focus_pct = round(focus/tracked×100) (0 se
// tracked=0); avg_session = tracked/nEntries (0 se 0 entries); untracked vem
// do availability. Sem NaN/div-zero.
func kpis(trackedSeconds, focusSeconds, nEntries, untrackedSeconds int) Kpis {
	deep := 0
	if trackedSeconds > 0 {
		deep = int(math.Round(float64(focusSeconds) / float64(trackedSeconds) * 100))
	}
	avg := 0
	if nEntries > 0 {
		avg = int(math.Round(float64(trackedSeconds) / float64(nEntries)))
	}
	return Kpis{
		TrackedSeconds:    trackedSeconds,
		DeepFocusPct:      deep,
		AvgSessionSeconds: avg,
		UntrackedSeconds:  untrackedSeconds,
	}
}
