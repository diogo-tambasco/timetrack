package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"

	"timetrack.live/backend/internal/api"
	"timetrack.live/backend/internal/auth"
	"timetrack.live/backend/internal/auth/googlestub"
	"timetrack.live/backend/internal/calendar"
	"timetrack.live/backend/internal/calendar/calendarstub"
	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/workspace"
)

func newCalendarRouter(t *testing.T, pool *pgxpool.Pool) (http.Handler, *auth.Service, *googlestub.Server) {
	t.Helper()
	svc, stub := newAuthService(t, pool)
	cal := calendarstub.New()
	t.Cleanup(cal.Close)
	calSvc := calendar.NewService(pool, oauth2.Config{
		ClientID:     "client-id-teste",
		ClientSecret: "client-secret-teste",
		Endpoint:     stub.Endpoint(), // refresh via googlestub (não usado com token válido)
	}, []byte("0123456789abcdef0123456789abcdef"), cal.URL())
	router := api.NewRouter(
		api.WithAuth(svc, workspace.NewStore(pool)),
		api.WithCalendar(calSvc),
	)
	return router, svc, stub
}

// loginViaCalendar roda o callback com o code de calendar → sessão com o
// token_calendar.json (scope calendar.readonly + access válido ~1h).
func loginViaCalendar(t *testing.T, svc *auth.Service, stub *googlestub.Server, sub, email string) *http.Cookie {
	t.Helper()
	stub.SetUserinfo(fmt.Appendf(nil,
		`{"sub":%q,"email":%q,"email_verified":true,"name":"Cal","picture":""}`, sub, email))
	req := httptest.NewRequest(http.MethodGet,
		"/auth/google/callback?code="+googlestub.CodeCalendar+"&state=ok", nil)
	req.AddCookie(&http.Cookie{Name: "tt_oauth_state", Value: "ok"})
	rec := httptest.NewRecorder()
	svc.HandleCallback(rec, req)
	for _, c := range rec.Result().Cookies() {
		if c.Name == "tt_session" && c.Value != "" {
			return c
		}
	}
	t.Fatalf("login calendar de %s não devolveu sessão (status %d: %s)", email, rec.Code, rec.Body.String())
	return nil
}

// CA-01/02/01.1: connect redireciona ao consentimento com calendar+prompt;
// sem sessão → 401; o login normal NÃO força prompt=consent.
func TestCalendarConnect(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCalendarRouter(t, pool)
	sess := loginVia(t, svc, stub, "g-cc", "cc@example.com", "CC")

	// CA-01: com sessão → 302 pro Google com scope de calendar + prompt + offline
	req := httptest.NewRequest(http.MethodGet, "/api/v1/calendar/connect", nil)
	req.AddCookie(sess)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("connect com sessão: status %d", rec.Code)
	}
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if !strings.Contains(loc.Query().Get("scope"), "calendar.readonly") {
		t.Errorf("connect sem o scope de calendar: %q", loc.Query().Get("scope"))
	}
	if loc.Query().Get("prompt") != "consent" {
		t.Errorf("connect sem prompt=consent: %q", loc.Query().Get("prompt"))
	}
	if loc.Query().Get("access_type") != "offline" {
		t.Errorf("connect sem access_type=offline: %q", loc.Query().Get("access_type"))
	}

	// CA-02: sem sessão → 401
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/calendar/connect", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("connect sem sessão: esperado 401, veio %d", rec.Code)
	}

	// CA-01.1: o login normal NÃO força prompt=consent
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/auth/google/login", nil))
	locLogin, _ := url.Parse(rec.Header().Get("Location"))
	if locLogin.Query().Get("prompt") != "" {
		t.Errorf("login não devia forçar prompt: %q", locLogin.Query().Get("prompt"))
	}
}

// CA-03/03.1: o callback de connect persiste o scope de calendar e volta pra
// /calendario; sem cookie de retorno volta pra /; retorno hostil é ignorado.
func TestCallbackRetornoEScopeCalendar(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCalendarRouter(t, pool)

	// connect (com cookie de retorno /calendario) + code-calendar
	stub.SetUserinfo([]byte(`{"sub":"g-ret","email":"ret@example.com","email_verified":true,"name":"Ret","picture":""}`))
	req := httptest.NewRequest(http.MethodGet,
		"/auth/google/callback?code="+googlestub.CodeCalendar+"&state=ok", nil)
	req.AddCookie(&http.Cookie{Name: "tt_oauth_state", Value: "ok"})
	req.AddCookie(&http.Cookie{Name: "tt_oauth_return", Value: "/calendario"})
	rec := httptest.NewRecorder()
	svc.HandleCallback(rec, req)
	if got := rec.Header().Get("Location"); got != "/calendario" {
		t.Errorf("callback de connect devia voltar pra /calendario, veio %q", got)
	}
	var sess *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "tt_session" && c.Value != "" {
			sess = c
		}
	}
	if sess == nil {
		t.Fatal("callback não devolveu sessão")
	}
	// status agora conectado (scope persistido)
	sreq := httptest.NewRequest(http.MethodGet, "/api/v1/calendar/status", nil)
	sreq.AddCookie(sess)
	srec := httptest.NewRecorder()
	router.ServeHTTP(srec, sreq)
	var st struct {
		Connected bool `json:"connected"`
	}
	_ = json.Unmarshal(srec.Body.Bytes(), &st)
	if !st.Connected {
		t.Errorf("status após connect devia ser connected:true, veio %s", srec.Body.String())
	}

	// CA-03.1: sem cookie de retorno → / ; retorno hostil ("//evil") → /
	for _, ret := range []string{"", "//evil.com"} {
		req := httptest.NewRequest(http.MethodGet,
			"/auth/google/callback?code="+googlestub.CodeOK+"&state=ok", nil)
		req.AddCookie(&http.Cookie{Name: "tt_oauth_state", Value: "ok"})
		if ret != "" {
			req.AddCookie(&http.Cookie{Name: "tt_oauth_return", Value: ret})
		}
		stub.SetUserinfo([]byte(`{"sub":"g-base","email":"base@example.com","email_verified":true,"name":"B","picture":""}`))
		rec := httptest.NewRecorder()
		svc.HandleCallback(rec, req)
		if got := rec.Header().Get("Location"); got != "/" {
			t.Errorf("retorno %q devia cair em /, veio %q", ret, got)
		}
	}
}

// CA-05/06/11: events conectado → 200; não conectado → 409; tz inválida → 400.
func TestCalendarEventsHTTP(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCalendarRouter(t, pool)

	// conectado (token_calendar)
	sess := loginViaCalendar(t, svc, stub, "g-ev", "ev@example.com")
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/calendar/events?from=2026-06-15&to=2026-06-21&tz=America/Sao_Paulo", nil)
	req.AddCookie(sess)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("events conectado: status %d (%s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Events []calendar.Event `json:"events"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Events) != 2 { // 1 timed + 1 all-day; cancelado filtrado
		t.Fatalf("esperava 2 eventos, veio %d: %+v", len(body.Events), body.Events)
	}

	// não conectado (login base) → 409 calendar_not_connected
	base := loginVia(t, svc, stub, "g-base2", "base2@example.com", "B")
	req = httptest.NewRequest(http.MethodGet, "/api/v1/calendar/events", nil)
	req.AddCookie(base)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "calendar_not_connected") {
		t.Errorf("não conectado: esperava 409 calendar_not_connected, veio %d %s", rec.Code, rec.Body.String())
	}

	// tz inválida → 400 validation_error
	req = httptest.NewRequest(http.MethodGet, "/api/v1/calendar/events?tz=Nowhere/Bad", nil)
	req.AddCookie(sess)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "validation_error") {
		t.Errorf("tz inválida: esperava 400 validation_error, veio %d %s", rec.Code, rec.Body.String())
	}
}

// Verificação do build (#1): no connect, a conta que consentiu DEVE bater com a
// sessão logada — senão o connect trocaria o usuário silenciosamente.
func TestConnectRejeitaContaDiferente(t *testing.T) {
	pool := dbtest.NewPool(t)
	_, svc, stub := newCalendarRouter(t, pool)

	sessA := loginVia(t, svc, stub, "conta-A", "a@example.com", "A")

	// callback de connect com a conta B (sub diferente) + sessão de A + cookie de retorno
	stub.SetUserinfo([]byte(`{"sub":"conta-B","email":"b@example.com","email_verified":true,"name":"B","picture":""}`))
	req := httptest.NewRequest(http.MethodGet,
		"/auth/google/callback?code="+googlestub.CodeCalendar+"&state=ok", nil)
	req.AddCookie(&http.Cookie{Name: "tt_oauth_state", Value: "ok"})
	req.AddCookie(&http.Cookie{Name: "tt_oauth_return", Value: "/calendario"})
	req.AddCookie(sessA)
	rec := httptest.NewRecorder()
	svc.HandleCallback(rec, req)

	if got := rec.Header().Get("Location"); got != "/calendario?erro=conta" {
		t.Errorf("conta divergente devia abortar pra /calendario?erro=conta, veio %q", got)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "tt_session" && c.Value != "" {
			t.Error("conta divergente NÃO devia recriar sessão")
		}
	}
	var n int
	_ = pool.QueryRow(context.Background(),
		`SELECT count(*) FROM users WHERE google_id = 'conta-B'`).Scan(&n)
	if n != 0 {
		t.Error("conta B não devia ter sido criada (upsert abortado)")
	}
}

// Verificação do build (#2): o cookie de retorno é expirado em TODO callback,
// inclusive nos de falha (senão regride o próximo login pra /calendario).
func TestCallbackFalhoLimpaCookieDeRetorno(t *testing.T) {
	pool := dbtest.NewPool(t)
	_, svc, _ := newCalendarRouter(t, pool)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?error=access_denied", nil)
	req.AddCookie(&http.Cookie{Name: "tt_oauth_return", Value: "/calendario"})
	rec := httptest.NewRecorder()
	svc.HandleCallback(rec, req)

	cleared := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == "tt_oauth_return" && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("callback de falha devia EXPIRAR o tt_oauth_return")
	}
}
