package workspace_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/workspace"
)

// Finding confirmado na verificação adversarial do bloco 3 da fase 02:
// o E2E roda fullyParallel com o MESMO usuário do stub, e em banco zerado
// dois callbacks simultâneos de 1º login caíam juntos no INSERT de users
// (unique_violation em google_id → 500). O upsert DEVE ser serializado
// por google_id: todos os logins concorrentes terminam sem erro, com
// exatamente 1 user e 1 workspace pessoal criados.
func TestPrimeiroLoginConcorrente(t *testing.T) {
	pool := dbtest.NewPool(t)
	store := workspace.NewStore(pool)

	perfil := workspace.GoogleProfile{
		GoogleID: "g-corrida",
		Email:    "corrida@example.com",
		Name:     "Corrida Concorrente",
	}
	tok := workspace.EncryptedToken{
		AccessTokenEnc: []byte("access-cifrado"),
		ExpiresAt:      time.Now().Add(time.Hour),
		Scopes:         []string{"openid"},
	}

	const n = 6
	largada := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, n)
	ids := make([]string, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-largada // todos disparam juntos, maximizando a colisão
			ids[i], errs[i] = store.UpsertUserFromGoogle(context.Background(), perfil, tok)
		}()
	}
	close(largada)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("login concorrente %d falhou: %v", i, err)
		}
	}
	for i := 1; i < n; i++ {
		if ids[i] != ids[0] {
			t.Fatalf("logins concorrentes devolveram users diferentes: %q vs %q", ids[0], ids[i])
		}
	}
	if got := dbtest.CountRows(t, pool, "users"); got != 1 {
		t.Errorf("esperado 1 user, existem %d", got)
	}
	if got := dbtest.CountRows(t, pool, "workspaces"); got != 1 {
		t.Errorf("esperado 1 workspace pessoal, existem %d", got)
	}
	if got := dbtest.CountRows(t, pool, "workspace_members"); got != 1 {
		t.Errorf("esperada 1 membership owner, existem %d", got)
	}
}
