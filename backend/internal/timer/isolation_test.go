// isolation_test.go — correções da verificação adversarial do bloco 1.
// F1: timers SIMULTÂNEOS de 2 usuários — Stop/Pause/Start de um nunca toca
// o estado do outro (CA-03 "por usuário" / CA-03.1). Mata as mutações que
// removem o filtro user_id de Stop, closeRunning e loadStateRow.
// F2: a tradução pomodoro_focus→"pomodoro" (apiModeOf) exercitada nos DOIS
// caminhos que a usam (State e resume) — plan §3.2 / CA-34.1.
// + clamps "nunca negativo" (CA-12) e do alvo retomado (CA-10) sob overrun.
package timer_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/timer"
)

// F1 — Ana e Bia com timers rodando AO MESMO TEMPO: cada mutação de timer
// enxerga só o estado do próprio usuário.
func TestIsolamentoTimerEntreUsuarios(t *testing.T) {
	svc, clock, pool, ana, wsID, _ := newTimerService(t)
	ctx := context.Background()

	var bia string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-bia', 'bia@tt.dev', 'Bia') RETURNING id`).Scan(&bia); err != nil {
		t.Fatalf("seed bia: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'member')`,
		wsID, bia); err != nil {
		t.Fatalf("membership da bia: %v", err)
	}

	// Bia começa primeiro; o start da Ana NÃO pode fechar a entry da Bia
	// (mata closeRunning sem filtro de usuário)
	biaRes, err := svc.Start(ctx, bia, timer.StartInput{WorkspaceID: wsID, Description: "da bia"})
	if err != nil {
		t.Fatalf("start da bia: %v", err)
	}
	clock.Advance(50 * time.Second)
	if _, err := svc.Start(ctx, ana, timer.StartInput{WorkspaceID: wsID, Description: "da ana"}); err != nil {
		t.Fatalf("start da ana: %v", err)
	}
	var biaAberta bool
	if err := pool.QueryRow(ctx,
		`SELECT stopped_at IS NULL FROM time_entries WHERE id = $1`,
		biaRes.State.Entry.ID).Scan(&biaAberta); err != nil || !biaAberta {
		t.Fatalf("o start da Ana NÃO pode fechar a entry da Bia (aberta=%v err=%v)", biaAberta, err)
	}

	// o stop da Ana não toca o timer da Bia (mata DELETE sem WHERE e
	// loadStateRow sem filtro)
	clock.Advance(100 * time.Second)
	if _, err := svc.Stop(ctx, ana); err != nil {
		t.Fatalf("stop da ana: %v", err)
	}
	if n := dbtest.CountRows(t, pool, "timer_states"); n != 1 {
		t.Fatalf("só o estado da Ana deveria sumir no stop dela; sobraram %d linhas", n)
	}
	biaSt, err := svc.State(ctx, bia)
	if err != nil || biaSt.State == nil || biaSt.State.Mode != "countup" {
		t.Fatalf("timer da Bia deveria seguir rodando após o stop da Ana (err=%v state=%+v)", err, biaSt.State)
	}
	if biaSt.State.Entry.ID != biaRes.State.Entry.ID || biaSt.State.ElapsedSeconds != 150 {
		t.Errorf("estado da Bia corrompido: entry=%s elapsed=%d (esperava a dela com 150s)",
			biaSt.State.Entry.ID, biaSt.State.ElapsedSeconds)
	}

	// o pause da Ana (novo timer) também não toca a Bia
	if _, err := svc.Start(ctx, ana, timer.StartInput{WorkspaceID: wsID, Description: "de novo"}); err != nil {
		t.Fatalf("restart da ana: %v", err)
	}
	clock.Advance(60 * time.Second)
	if _, err := svc.Pause(ctx, ana, nil); err != nil {
		t.Fatalf("pause da ana: %v", err)
	}
	biaSt, err = svc.State(ctx, bia)
	if err != nil || biaSt.State == nil || biaSt.State.Mode != "countup" {
		t.Fatalf("o pause da Ana virou o estado da Bia (err=%v state=%+v)", err, biaSt.State)
	}
	if biaAberta := runningOf(t, pool, bia); biaAberta != 1 {
		t.Errorf("a entry da Bia deveria seguir rodando, vieram %d rodando", biaAberta)
	}

	// e o stop da Bia fecha SÓ a dela, com a duração dela (210s)
	res, err := svc.Stop(ctx, bia)
	if err != nil {
		t.Fatalf("stop da bia: %v", err)
	}
	if res.StoppedEntry == nil || *res.StoppedEntry.DurationSeconds != 210 {
		t.Errorf("duração da Bia esperada 210s, veio %v", res.StoppedEntry.DurationSeconds)
	}
	anaSt, err := svc.State(ctx, ana)
	if err != nil || anaSt.State == nil || anaSt.State.Mode != "break" {
		t.Errorf("a pausa da Ana deveria seguir intacta após o stop da Bia (err=%v state=%+v)", err, anaSt.State)
	}
}

func runningOf(t *testing.T, pool *pgxpool.Pool, userID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM time_entries WHERE user_id = $1 AND stopped_at IS NULL`,
		userID).Scan(&n); err != nil {
		t.Fatalf("contar rodando: %v", err)
	}
	return n
}

// F2 — o ciclo pomodoro inteiro (State rodando → pause → resume) responde
// "pomodoro" no state.mode (nunca o enum interno) e 'pomodoro_focus' SÓ no
// resume.mode (contrato do CA-34.1).
func TestPomodoroNaoVazaEnumInternoNoCiclo(t *testing.T) {
	svc, clock, pool, userID, wsID, _ := newTimerService(t)
	ctx := context.Background()

	if _, err := svc.SaveSettings(ctx, userID, timer.Settings{FocusMinutes: 25, BreakMinutes: 5}); err != nil {
		t.Fatalf("settings: %v", err)
	}
	if _, err := svc.Start(ctx, userID, timer.StartInput{WorkspaceID: wsID, Mode: "pomodoro"}); err != nil {
		t.Fatalf("start pomodoro: %v", err)
	}

	// State com pomodoro RODANDO traduz o enum (mata apiModeOf identidade
	// no stateOf)
	clock.Advance(300 * time.Second)
	st, err := svc.State(ctx, userID)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if st.State.Mode != "pomodoro" {
		t.Errorf("state.mode do GET deveria ser 'pomodoro', veio %q (enum interno vazou)", st.State.Mode)
	}
	if st.State.RemainingSeconds == nil || *st.State.RemainingSeconds != 1200 {
		t.Errorf("remaining esperado 1200 (1500−300), veio %v", st.State.RemainingSeconds)
	}

	// na pausa, o contexto de retomada expõe o valor INTERNO (CA-34.1)...
	pause, err := svc.Pause(ctx, userID, nil)
	if err != nil {
		t.Fatalf("pause: %v", err)
	}
	if pause.State.Resume == nil || pause.State.Resume.Mode != "pomodoro_focus" {
		t.Errorf("resume.mode deveria ser 'pomodoro_focus' (o front decide o auto-resume por ele), veio %+v",
			pause.State.Resume)
	}
	if pause.State.Resume.TargetSeconds == nil || *pause.State.Resume.TargetSeconds != 1200 {
		t.Errorf("restante do foco esperado 1200, veio %v", pause.State.Resume.TargetSeconds)
	}

	// ...e o resume volta traduzido (mata apiModeOf identidade no
	// resumeFromBreak), restaurando o restante do alvo
	clock.Advance(30 * time.Second)
	res, err := svc.Resume(ctx, userID)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if res.State.Mode != "pomodoro" {
		t.Errorf("state.mode pós-resume deveria ser 'pomodoro', veio %q", res.State.Mode)
	}
	if res.State.TargetSeconds == nil || *res.State.TargetSeconds != 1200 {
		t.Errorf("alvo retomado esperado 1200, veio %v", res.State.TargetSeconds)
	}
	var dbMode string
	if err := pool.QueryRow(ctx,
		`SELECT mode::text FROM timer_states WHERE user_id = $1`, userID).Scan(&dbMode); err != nil || dbMode != "pomodoro_focus" {
		t.Errorf("no banco o modo retomado é 'pomodoro_focus', veio %q (err=%v)", dbMode, err)
	}
}

// CA-12 ("nunca negativo") + CA-10 sob OVERRUN: alvo vencido reporta
// remaining 0 e o restante guardado na pausa clampa em 0 — nunca negativo.
func TestOverrunClampaRemainingEAlvoRetomado(t *testing.T) {
	svc, clock, _, userID, wsID, _ := newTimerService(t)
	ctx := context.Background()

	target := 300
	if _, err := svc.Start(ctx, userID, timer.StartInput{
		WorkspaceID: wsID, Mode: "countdown", TargetSeconds: &target,
	}); err != nil {
		t.Fatalf("start: %v", err)
	}
	clock.Advance(400 * time.Second) // 100s além do alvo

	st, err := svc.State(ctx, userID)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if st.State.RemainingSeconds == nil || *st.State.RemainingSeconds != 0 {
		t.Errorf("remaining com alvo vencido deveria clampar em 0, veio %v", st.State.RemainingSeconds)
	}

	pause, err := svc.Pause(ctx, userID, nil)
	if err != nil {
		t.Fatalf("pause: %v", err)
	}
	if pause.State.Resume.TargetSeconds == nil || *pause.State.Resume.TargetSeconds != 0 {
		t.Errorf("restante guardado pós-overrun deveria ser 0 (nunca negativo), veio %v",
			pause.State.Resume.TargetSeconds)
	}
}
