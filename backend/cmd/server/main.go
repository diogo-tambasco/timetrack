// Servidor do Timetrack. Boot: config → conexão (retry) → migrações →
// router → listener. Nenhuma requisição é atendida com migração pendente.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"
	// o recorte "hoje" por fuso IANA (CA-14.1 da fase 03) precisa do banco
	// de timezones mesmo no container distroless — embute no binário
	_ "time/tzdata"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"

	"timetrack.live/backend/internal/api"
	"timetrack.live/backend/internal/auth"
	"timetrack.live/backend/internal/calendar"
	"timetrack.live/backend/internal/clients"
	"timetrack.live/backend/internal/comments"
	"timetrack.live/backend/internal/config"
	"timetrack.live/backend/internal/db"
	"timetrack.live/backend/internal/entries"
	"timetrack.live/backend/internal/guard"
	"timetrack.live/backend/internal/invitations"
	"timetrack.live/backend/internal/notes"
	"timetrack.live/backend/internal/objectives"
	"timetrack.live/backend/internal/presence"
	"timetrack.live/backend/internal/projects"
	"timetrack.live/backend/internal/properties"
	"timetrack.live/backend/internal/push"
	"timetrack.live/backend/internal/realtime"
	"timetrack.live/backend/internal/reminders"
	"timetrack.live/backend/internal/reports"
	"timetrack.live/backend/internal/subtasks"
	"timetrack.live/backend/internal/tasks"
	"timetrack.live/backend/internal/timer"
	"timetrack.live/backend/internal/workspace"
)

// Options ajusta o Run pra testes (timeout curto, descoberta de porta).
type Options struct {
	ConnectTimeout time.Duration   // default 30s (CA-04)
	Ready          chan<- net.Addr // recebe o endereço real após o listener abrir
}

// Run sobe o servidor e bloqueia até ctx ser cancelado ou erro fatal.
func Run(ctx context.Context, cfg config.Config, logger *slog.Logger, opts Options) error {
	timeout := opts.ConnectTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	pool, err := db.Connect(ctx, cfg.DatabaseURL, timeout)
	if err != nil {
		logger.Error("boot abortado: postgres indisponível", "err", err)
		return err
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, db.Migrations()); err != nil {
		logger.Error("boot abortado: migração falhou", "err", err)
		return err
	}
	logger.Info("migrações aplicadas")

	staticFS := api.EmbeddedDist()
	if cfg.StaticDir != "" {
		staticFS = os.DirFS(cfg.StaticDir)
	}

	// fase 17: convites + gestão de membros. O ResolveForEmail é injetado no
	// auth.Config (D-05) pra materializar convites pendentes no 1º login —
	// construído ANTES do authSvc (que recebe a função como hook).
	invSvc := invitations.NewService(pool, invitations.WithTTL(cfg.InviteTTL))

	authSvc := auth.NewService(auth.Config{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  cfg.OAuthRedirectURL,
		CookieSecure: cfg.CookieSecure,
		TokenKey:     cfg.TokenKey,
		// vazios → NewService usa o Google real; E2E injeta o stub por env
		Endpoint:    oauth2.Endpoint{AuthURL: cfg.GoogleAuthURL, TokenURL: cfg.GoogleTokenURL},
		UserinfoURL: cfg.GoogleUserinfoURL,
		DB:          pool,
		// resolução de convites no callback de auth (NÃO no NewStore — D-05)
		ResolveInvites: invSvc.ResolveForEmail,
	})
	// fase 04: broadcast do timer por usuário — toda mutação commitada chega
	// a todas as conexões /ws do dono (plan §3.3).
	// fase 21: presence ao vivo por workspace. O OnChange vira um hook COMPOSTO
	// (D-04): BroadcastTimer (fase 04) PRIMEIRO — o sync do dono não pode esperar
	// — e presenceSvc.OnTimer (fase 21) DEPOIS, aditivo e best-effort (nunca
	// falha). O MESMO hook composto é usado no guard.WithOnTimer (cutter da 16)
	// pra o corte automático também derrubar a presence (CA-09/D-04).
	tsvc := timer.NewService(pool)
	hub := realtime.NewHub()
	presenceSvc := presence.NewService(pool, hub.BroadcastPresence)
	onTimer := composeOnTimer(hub.BroadcastTimer, presenceSvc.OnTimer)
	tsvc.SetOnChange(onTimer)

	// fase 06: leitura do Google Calendar. O refresh do token usa o mesmo
	// endpoint OAuth do authSvc (Google real em prod, stub por env no E2E).
	googleEndpoint := oauth2.Endpoint{AuthURL: cfg.GoogleAuthURL, TokenURL: cfg.GoogleTokenURL}
	if googleEndpoint.TokenURL == "" {
		googleEndpoint = endpoints.Google
	}
	calSvc := calendar.NewService(pool, oauth2.Config{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		Endpoint:     googleEndpoint,
	}, cfg.TokenKey, cfg.GoogleCalendarAPIURL)

	// fase 16: anti-timer-esquecido. push.Sender (best-effort, no-op sem
	// VAPID) + guard.Cutter (mesmo advisory lock do timer; onTimer=BroadcastTimer
	// faz a UI ficar ociosa como num Stop remoto, onCut dispara o aviso humano
	// — notice in-app + push pós-commit, fora do lock). O worker tica no ctx do
	// Run e para no shutdown (CA-18).
	pushSubs := push.NewSubscriptions(pool)
	pushSender := push.NewSender(pushSubs, cfg.VapidPublicKey, cfg.VapidPrivateKey, cfg.VapidSubject, logger)
	cutter := guard.NewCutter(pool,
		guard.WithOnTimer(onTimer), // fase 21 (D-04): o corte da 16 também derruba a presence
		guard.WithOnCut(func(evt guard.CutEvent) {
			hub.BroadcastNotice(evt.UserID, noticePayload(evt))
			go pushSender.SendToUser(context.Background(), evt.UserID, pushPayload())
		}),
	)
	worker := guard.NewWorker(cutter, cfg.GuardTick, logger)
	go worker.Run(ctx)

	// fase 20: lembretes (due/manual). Worker PRÓPRIO (espelha o guard, NÃO o
	// importa): o Dispatcher reusa o hub.BroadcastNotice (notice in-app) + o
	// pushSender.SendToUser (Web Push best-effort, pós-commit, em goroutine). O
	// reminder due nasce na escrita da task → injeta o ReminderHook no tasks.Service
	// ANTES de montar o router. Sequência de boot 18→20→21 (a 18 já está no
	// tasks.Service; a 20 só adiciona o hook + worker). REMINDER_TICK opcional
	// (default 60s); o worker tica no ctx do Run e para no shutdown (CA-29).
	tasksSvc := tasks.NewService(pool)
	reminderStore := reminders.NewStore(pool)
	tasksSvc.SetReminderHook(reminders.NewHook(reminderStore))
	reminderDispatcher := reminders.NewDispatcher(pool,
		reminders.WithOnNotice(hub.BroadcastNotice),
		reminders.WithOnPush(func(u string, p []byte) {
			go pushSender.SendToUser(context.Background(), u, p)
		}),
	)
	reminderWorker := reminders.NewWorker(reminderDispatcher, cfg.ReminderTick, logger)
	go reminderWorker.Run(ctx)

	router := api.NewRouter(
		api.WithStaticFS(staticFS),
		api.WithAuth(authSvc, workspace.NewStore(pool)),
		api.WithCore(api.Core{
			Clients:    clients.NewService(pool),
			Projects:   projects.NewService(pool),
			Tasks:      tasksSvc,
			Objectives: objectives.NewService(pool),
			Subtasks:   subtasks.NewService(pool),   // fase 24: sub-tasks "goals da task"
			Properties: properties.NewService(pool), // fase 25: propriedades tipadas custom
			// fase 18: atribuir uma task a um TERCEIRO avisa o assignee — notice
			// in-app (WebSocket da 04) + Web Push (fase 16), pós-commit. Espelha o
			// onCut do guard: notice síncrono no hub, push em goroutine best-effort.
			OnAssign: func(target, taskID, by string) {
				hub.BroadcastNotice(target, map[string]any{
					"kind": "task_assigned", "task_id": taskID, "by": by,
				})
				go pushSender.SendToUser(context.Background(), target, taskAssignedPushPayload())
			},
		}),
		api.WithTimer(tsvc),
		api.WithEntries(entries.NewService(pool)),
		api.WithReports(reports.NewService(pool)),
		api.WithRealtime(hub),
		api.WithPresence(presenceSvc), // fase 21: snapshot GET /workspaces/{id}/presence
		api.WithCalendar(calSvc),
		api.WithPush(api.Push{Subs: pushSubs, Sender: pushSender}),
		api.WithInvitations(api.Invitations{Service: invSvc}),
		api.WithComments(api.Comments{Svc: comments.NewService(pool)}),             // fase 19: thread em tasks
		api.WithReminders(api.Reminders{Svc: reminders.NewService(reminderStore)}), // fase 20: lembretes
		api.WithNotes(api.Notes{Svc: notes.NewService(pool, tasksSvc)}),            // fase 26: notas nativas .md
	)

	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		logger.Error("boot abortado: listener", "err", err)
		return err
	}
	if opts.Ready != nil {
		opts.Ready <- ln.Addr()
	}
	logger.Info("servindo", "addr", ln.Addr().String())

	srv := &http.Server{Handler: router}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		logger.Error("servidor caiu", "err", err)
		return err
	}
}

// composeOnTimer encadeia o hook do timer (fase 04: BroadcastTimer) com o do
// presence (fase 21: presence.OnTimer) num único func(userID, st, at) (D-04). O
// `first` (BroadcastTimer) roda SEMPRE primeiro — o sync do dono não pode
// esperar o fan-out de presence; o `then` (OnTimer) é aditivo e best-effort
// (engole o próprio erro). É usado TANTO no tsvc.SetOnChange QUANTO no
// guard.WithOnTimer (o corte da 16 derruba a presence de graça). Extraído pra
// um helper testável (T-06) sem duplicar o wiring.
func composeOnTimer(
	first func(userID string, st *timer.State, at time.Time),
	then func(userID string, st *timer.State, at time.Time),
) func(userID string, st *timer.State, at time.Time) {
	return func(userID string, st *timer.State, at time.Time) {
		first(userID, st, at)
		then(userID, st, at)
	}
}

// noticePayload monta o payload do aviso in-app (CA-10/12): kind + entry +
// marco do corte. O banner do front identifica o aviso por kind.
func noticePayload(evt guard.CutEvent) map[string]any {
	return map[string]any{
		"kind":       "timer_auto_cut",
		"entry_id":   evt.EntryID,
		"stopped_at": evt.StoppedAt.Format(time.RFC3339),
	}
}

// pushPayload monta o corpo da notificação Web Push do corte (CA-15). É o
// JSON que o service worker recebe no evento `push` e mostra via
// showNotification.
func pushPayload() []byte {
	b, _ := json.Marshal(push.Payload{
		Title: "Timer pausado",
		Body:  "Seu timer foi pausado automaticamente após 2h.",
		Kind:  "timer_auto_cut",
	})
	return b
}

// taskAssignedPushPayload monta o corpo do Web Push da atribuição (fase 18,
// CA-13). Mesma mecânica do pushPayload do corte.
func taskAssignedPushPayload() []byte {
	b, _ := json.Marshal(push.Payload{
		Title: "Nova task pra você",
		Body:  "Alguém te atribuiu uma task no Timetrack.",
		Kind:  "task_assigned",
	})
	return b
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		logger.Error("configuração inválida", "err", err)
		os.Exit(1)
	}

	if err := Run(context.Background(), cfg, logger, Options{}); err != nil {
		os.Exit(1)
	}
}
