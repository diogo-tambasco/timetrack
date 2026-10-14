package calendar_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"

	"timetrack.live/backend/internal/auth"
	"timetrack.live/backend/internal/calendar"
	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/domain"
	"timetrack.live/backend/internal/workspace"
)

func testKey() []byte { return []byte("0123456789abcdef0123456789abcdef") }

func oauthCfg(base string) oauth2.Config {
	return oauth2.Config{
		ClientID:     "cid",
		ClientSecret: "csec",
		Endpoint:     oauth2.Endpoint{TokenURL: base + "/token", AuthURL: base + "/authorize"},
	}
}

// fakeGoogle serve o /calendars/primary/events e o /token (refresh) — o
// googlestub não trata refresh (armadilha §6.9 do plan).
type fakeGoogle struct {
	srv           *httptest.Server
	mu            sync.Mutex
	events        string
	eventsStatus  int
	refreshAccess string // novo access no refresh; "" → invalid_grant
	gotAuth       string
	tokenHits     int
}

func newFakeGoogle(t *testing.T) *fakeGoogle {
	t.Helper()
	f := &fakeGoogle{eventsStatus: http.StatusOK, events: `{"items":[]}`}
	mux := http.NewServeMux()
	mux.HandleFunc("/calendars/primary/events", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.gotAuth = r.Header.Get("Authorization")
		status, body := f.eventsStatus, f.events
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status == http.StatusOK {
			_, _ = io.WriteString(w, body)
		} else {
			_, _ = io.WriteString(w, `{"error":{"message":"boom"}}`)
		}
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.tokenHits++
		access := f.refreshAccess
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if access == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":"invalid_grant"}`)
			return
		}
		_, _ = io.WriteString(w, fmt.Sprintf(
			`{"access_token":%q,"token_type":"Bearer","expires_in":3600}`, access))
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeGoogle) URL() string { return f.srv.URL }
func (f *fakeGoogle) auth() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.gotAuth
}
func (f *fakeGoogle) refreshes() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tokenHits
}

// seedUser cria um user (via o upsert real) com token cifrado pelo auth.Seal.
func seedUser(t *testing.T, pool *pgxpool.Pool, key []byte, sub, access, refresh string, scopes []string, expiry time.Time) string {
	t.Helper()
	store := workspace.NewStore(pool)
	accEnc, err := auth.Seal(key, []byte(access))
	if err != nil {
		t.Fatalf("seal access: %v", err)
	}
	var refEnc []byte
	if refresh != "" {
		if refEnc, err = auth.Seal(key, []byte(refresh)); err != nil {
			t.Fatalf("seal refresh: %v", err)
		}
	}
	id, err := store.UpsertUserFromGoogle(context.Background(),
		workspace.GoogleProfile{GoogleID: sub, Email: sub + "@example.com", Name: "Cal"},
		workspace.EncryptedToken{
			AccessTokenEnc:  accEnc,
			RefreshTokenEnc: refEnc,
			ExpiresAt:       expiry,
			Scopes:          scopes,
		})
	if err != nil {
		t.Fatalf("seedUser: %v", err)
	}
	return id
}

func hasScope(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want {
			return true
		}
	}
	return false
}

func assertDomain(t *testing.T, err error, status int, code string) {
	t.Helper()
	var de *domain.Error
	if !errors.As(err, &de) {
		t.Fatalf("esperava *domain.Error, veio %T: %v", err, err)
	}
	if de.Status != status || de.Code != code {
		t.Fatalf("esperava %d/%s, veio %d/%s", status, code, de.Status, de.Code)
	}
}

func week(loc *time.Location) (time.Time, time.Time) {
	from := time.Date(2026, 6, 15, 0, 0, 0, 0, loc) // segunda
	return from, from.AddDate(0, 0, 7)
}

// CA-05/07/08: eventos com horário e all-day mapeados; cancelado omitido.
func TestEventsMapeiaTimedAllDayEOmiteCancelado(t *testing.T) {
	pool := dbtest.NewPool(t)
	key := testKey()
	f := newFakeGoogle(t)
	f.events = `{"items":[
		{"id":"e1","summary":"Reunião","status":"confirmed","htmlLink":"https://cal/e1",
		 "start":{"dateTime":"2026-06-15T09:00:00-03:00"},"end":{"dateTime":"2026-06-15T10:00:00-03:00"}},
		{"id":"e2","summary":"Feriado","status":"confirmed","htmlLink":"https://cal/e2",
		 "start":{"date":"2026-06-17"},"end":{"date":"2026-06-18"}},
		{"id":"e3","summary":"Cancelado","status":"cancelled",
		 "start":{"dateTime":"2026-06-15T11:00:00-03:00"},"end":{"dateTime":"2026-06-15T12:00:00-03:00"}}
	]}`
	id := seedUser(t, pool, key, "g-map", "tok", "ref",
		[]string{"openid", calendar.ReadonlyScope}, time.Now().Add(time.Hour))
	svc := calendar.NewService(pool, oauthCfg(f.URL()), key, f.URL())

	loc, _ := time.LoadLocation("America/Sao_Paulo")
	from, to := week(loc)
	evs, err := svc.Events(context.Background(), id, from, to)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(evs) != 2 {
		t.Fatalf("esperava 2 eventos (cancelado fora), veio %d: %+v", len(evs), evs)
	}
	if evs[0].ID != "e1" || evs[0].AllDay {
		t.Errorf("e1 deveria ser timed: %+v", evs[0])
	}
	if evs[0].Start.Hour() != 9 {
		t.Errorf("e1 start hora errada: %v", evs[0].Start)
	}
	if evs[1].ID != "e2" || !evs[1].AllDay {
		t.Errorf("e2 deveria ser all-day: %+v", evs[1])
	}
	if y, m, d := evs[1].Start.In(loc).Date(); y != 2026 || m != 6 || d != 17 {
		t.Errorf("e2 start dia errado: %v", evs[1].Start)
	}
}

// CA-06: sem o scope de calendar → 409 calendar_not_connected (sem Google).
func TestEventsNaoConectado(t *testing.T) {
	pool := dbtest.NewPool(t)
	key := testKey()
	f := newFakeGoogle(t)
	id := seedUser(t, pool, key, "g-noscope", "tok", "ref",
		[]string{"openid", "email"}, time.Now().Add(time.Hour))
	svc := calendar.NewService(pool, oauthCfg(f.URL()), key, f.URL())

	from, to := week(time.UTC)
	_, err := svc.Events(context.Background(), id, from, to)
	assertDomain(t, err, http.StatusConflict, "calendar_not_connected")

	if conn, err := svc.Status(context.Background(), id); err != nil || conn {
		t.Fatalf("Status sem calendar devia ser false (conn=%v err=%v)", conn, err)
	}
}

func TestStatusConectado(t *testing.T) {
	pool := dbtest.NewPool(t)
	key := testKey()
	f := newFakeGoogle(t)
	id := seedUser(t, pool, key, "g-conn", "tok", "ref",
		[]string{"openid", calendar.ReadonlyScope}, time.Now().Add(time.Hour))
	svc := calendar.NewService(pool, oauthCfg(f.URL()), key, f.URL())
	if conn, err := svc.Status(context.Background(), id); err != nil || !conn {
		t.Fatalf("Status com calendar devia ser true (conn=%v err=%v)", conn, err)
	}
}

// CA-09: token expirado → refresh → re-persiste SÓ o access novo; scopes e
// refresh seguem byte-idênticos. Falsificável: access do refresh é DIFERENTE.
func TestEventsRenovaEPersisteAccess(t *testing.T) {
	pool := dbtest.NewPool(t)
	key := testKey()
	f := newFakeGoogle(t)
	f.refreshAccess = "ACCESS-NOVO-123"
	id := seedUser(t, pool, key, "g-refresh", "ACCESS-VELHO", "refresh-ok",
		[]string{"openid", calendar.ReadonlyScope}, time.Now().Add(-time.Hour)) // EXPIRADO
	svc := calendar.NewService(pool, oauthCfg(f.URL()), key, f.URL())

	from, to := week(time.UTC)
	if _, err := svc.Events(context.Background(), id, from, to); err != nil {
		t.Fatalf("Events: %v", err)
	}
	if f.refreshes() == 0 {
		t.Fatal("o refresh nunca foi chamado (token não estava expirado?)")
	}
	if f.auth() != "Bearer ACCESS-NOVO-123" {
		t.Errorf("o GET de eventos não usou o access novo: %q", f.auth())
	}

	st, _, _ := workspace.NewStore(pool).LoadGoogleToken(context.Background(), id)
	acc, _ := auth.Open(key, st.AccessTokenEnc)
	if string(acc) != "ACCESS-NOVO-123" {
		t.Errorf("access não foi re-persistido cifrado: %q", acc)
	}
	ref, _ := auth.Open(key, st.RefreshTokenEnc)
	if string(ref) != "refresh-ok" {
		t.Errorf("o refresh foi tocado no re-persist: %q", ref)
	}
	if !hasScope(st.Scopes, calendar.ReadonlyScope) {
		t.Errorf("os scopes foram tocados no re-persist: %v", st.Scopes)
	}
}

// CA-10: refresh revogado (invalid_grant) → 409 calendar_reconnect_required.
func TestEventsReconectaQuandoRefreshFalha(t *testing.T) {
	pool := dbtest.NewPool(t)
	key := testKey()
	f := newFakeGoogle(t)
	f.refreshAccess = "" // /token → 400 invalid_grant
	id := seedUser(t, pool, key, "g-revoked", "velho", "refresh-revogado",
		[]string{"openid", calendar.ReadonlyScope}, time.Now().Add(-time.Hour))
	svc := calendar.NewService(pool, oauthCfg(f.URL()), key, f.URL())

	from, to := week(time.UTC)
	_, err := svc.Events(context.Background(), id, from, to)
	assertDomain(t, err, http.StatusConflict, "calendar_reconnect_required")
}

// CA-10.1: Google 5xx → 502 calendar_upstream, NUNCA 500.
func TestEventsUpstream(t *testing.T) {
	pool := dbtest.NewPool(t)
	key := testKey()
	f := newFakeGoogle(t)
	f.eventsStatus = http.StatusInternalServerError
	id := seedUser(t, pool, key, "g-upstream", "tok", "ref",
		[]string{"openid", calendar.ReadonlyScope}, time.Now().Add(time.Hour))
	svc := calendar.NewService(pool, oauthCfg(f.URL()), key, f.URL())

	from, to := week(time.UTC)
	_, err := svc.Events(context.Background(), id, from, to)
	assertDomain(t, err, http.StatusBadGateway, "calendar_upstream")
}

// CA-12: a leitura usa SÓ o token do usuário da sessão.
func TestEventsIsolamentoPorUsuario(t *testing.T) {
	pool := dbtest.NewPool(t)
	key := testKey()
	f := newFakeGoogle(t)
	idA := seedUser(t, pool, key, "g-A", "TOKEN-A", "ref",
		[]string{"openid", calendar.ReadonlyScope}, time.Now().Add(time.Hour))
	seedUser(t, pool, key, "g-B", "TOKEN-B", "ref",
		[]string{"openid", calendar.ReadonlyScope}, time.Now().Add(time.Hour))
	svc := calendar.NewService(pool, oauthCfg(f.URL()), key, f.URL())

	from, to := week(time.UTC)
	if _, err := svc.Events(context.Background(), idA, from, to); err != nil {
		t.Fatalf("Events(A): %v", err)
	}
	if f.auth() != "Bearer TOKEN-A" {
		t.Errorf("Events(A) usou o token errado: %q", f.auth())
	}
}
