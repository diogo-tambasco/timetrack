package auth

import (
	"context"
	"fmt"
	"testing"

	"timetrack.live/backend/internal/auth/googlestub"
	"timetrack.live/backend/internal/db/dbtest"
)

// loginStubbado roda o callback completo com state válido (cookie = query).
func loginStubbado(t *testing.T, svc *Service) {
	t.Helper()
	rec := callbackRequest(t, svc, "code="+googlestub.CodeOK+"&state=state-valido", "state-valido")
	if rec.Code >= 500 {
		t.Fatalf("callback não pode estourar 5xx, veio %d: %s", rec.Code, rec.Body.String())
	}
}

func userinfoJSON(sub, email, name, picture string) []byte {
	return fmt.Appendf(nil,
		`{"sub":%q,"email":%q,"email_verified":true,"name":%q,"picture":%q}`,
		sub, email, name, picture)
}

// CA-04: 1º login cria, numa única transação, user + workspace pessoal
// "Workspace de <primeiro nome>" + membership owner. Bordas: name vazio →
// "Workspace pessoal"; mesmo email com google_id novo → re-vincula sem criar.
func TestPrimeiroLoginCriaWorkspaceOwner(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc, stub := stubServiceComDB(t, pool, false)
	ctx := context.Background()

	loginStubbado(t, svc) // userinfo.json: "Diogo Fixture"

	if n := dbtest.CountRows(t, pool, "users"); n != 1 {
		t.Fatalf("esperado 1 user, tem %d", n)
	}
	if n := dbtest.CountRows(t, pool, "workspaces"); n != 1 {
		t.Fatalf("esperado 1 workspace, tem %d", n)
	}

	var wsName, role string
	if err := pool.QueryRow(ctx, `
		SELECT w.name, m.role
		  FROM workspaces w
		  JOIN workspace_members m ON m.workspace_id = w.id
		  JOIN users u ON u.id = m.user_id
		 WHERE u.google_id = 'g-fixture-110248495921238986420'`).Scan(&wsName, &role); err != nil {
		t.Fatalf("membership do 1º login não existe: %v", err)
	}
	if wsName != "Workspace de Diogo" {
		t.Errorf("workspace esperado 'Workspace de Diogo', veio %q", wsName)
	}
	if role != "owner" {
		t.Errorf("role esperada owner, veio %q", role)
	}

	// borda: name vazio → fallback "Workspace pessoal" (D-10)
	stub.SetUserinfo(userinfoJSON("g-sem-nome", "sem.nome@example.com", "", ""))
	loginStubbado(t, svc)
	var wsName2 string
	if err := pool.QueryRow(ctx, `
		SELECT w.name FROM workspaces w
		  JOIN workspace_members m ON m.workspace_id = w.id
		  JOIN users u ON u.id = m.user_id
		 WHERE u.google_id = 'g-sem-nome'`).Scan(&wsName2); err != nil {
		t.Fatalf("workspace do user sem nome não existe: %v", err)
	}
	if wsName2 != "Workspace pessoal" {
		t.Errorf("fallback esperado 'Workspace pessoal', veio %q", wsName2)
	}

	// borda: mesmo email com google_id diferente (conta recriada) →
	// re-vincula o google_id ao user existente, sem criar user nem 500
	usersAntes := dbtest.CountRows(t, pool, "users")
	stub.SetUserinfo(userinfoJSON("g-recriado", "diogo.fixture@example.com", "Diogo Recriado", ""))
	loginStubbado(t, svc)
	if n := dbtest.CountRows(t, pool, "users"); n != usersAntes {
		t.Errorf("re-vínculo não pode criar user: antes %d, depois %d", usersAntes, n)
	}
	var revinculado bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE email = 'diogo.fixture@example.com' AND google_id = 'g-recriado')`).
		Scan(&revinculado); err != nil || !revinculado {
		t.Errorf("google_id deveria ter sido re-vinculado ao user do email (err=%v)", err)
	}
}

// CA-05: login repetido não duplica user/workspace; atualiza name/avatar_url.
func TestLoginRepetidoNaoDuplica(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc, stub := stubServiceComDB(t, pool, false)
	ctx := context.Background()

	loginStubbado(t, svc)
	users, workspaces := dbtest.CountRows(t, pool, "users"), dbtest.CountRows(t, pool, "workspaces")

	// 2º login do MESMO google_id com perfil atualizado
	stub.SetUserinfo(userinfoJSON("g-fixture-110248495921238986420",
		"diogo.fixture@example.com", "Diogo Atualizado", "https://lh3.googleusercontent.com/a/novo-avatar"))
	loginStubbado(t, svc)

	if n := dbtest.CountRows(t, pool, "users"); n != users {
		t.Errorf("users mudou de %d pra %d no login repetido", users, n)
	}
	if n := dbtest.CountRows(t, pool, "workspaces"); n != workspaces {
		t.Errorf("workspaces mudou de %d pra %d no login repetido", workspaces, n)
	}

	var name, avatar string
	if err := pool.QueryRow(ctx,
		`SELECT name, avatar_url FROM users WHERE google_id = 'g-fixture-110248495921238986420'`).
		Scan(&name, &avatar); err != nil {
		t.Fatal(err)
	}
	if name != "Diogo Atualizado" {
		t.Errorf("name deveria ter sido atualizado, veio %q", name)
	}
	if avatar != "https://lh3.googleusercontent.com/a/novo-avatar" {
		t.Errorf("avatar_url deveria ter sido atualizado, veio %q", avatar)
	}
}
