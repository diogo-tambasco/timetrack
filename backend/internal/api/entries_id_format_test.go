// entries_id_format_test.go — correção da verificação adversarial do
// bloco 2 (F4): uuid.Validate aceitava a forma URN ('urn:uuid:...'), que o
// Postgres rejeita no cast → 500 em vez do 404 uniforme do CA-29.3. O
// contrato do path é a forma canônica 8-4-4-4-12: qualquer outra coisa
// responde o MESMO not_found.
package api_test

import (
	"net/http"
	"testing"
)

func TestEntryIDForaDaFormaCanonicaE404(t *testing.T) {
	_, router, _, _, cookie, _, wsID := pauseSetup(t)

	ids := []string{
		"urn:uuid:6ba7b810-9dad-11d1-80b4-00c04fd430c8", // URN: Postgres rejeita o cast → era 500
		"{6ba7b810-9dad-11d1-80b4-00c04fd430c8}",        // braces
		"6ba7b8109dad11d180b400c04fd430c8",              // hex sem hífens
		"nao-uuid",
	}
	for _, id := range ids {
		for _, r := range []struct{ method, path, body string }{
			{http.MethodPatch, entriesPath(wsID) + "/" + id, `{"description":"x"}`},
			{http.MethodDelete, entriesPath(wsID) + "/" + id, ""},
			{http.MethodPost, entriesPath(wsID) + "/" + id + "/continue", ""},
		} {
			rec := doJSON(router, r.method, r.path, cookie, r.body)
			if rec.Code != http.StatusNotFound || errCode(t, rec) != "not_found" {
				t.Errorf("%s com id %q: esperava 404 not_found uniforme, veio %d %s",
					r.method, id, rec.Code, rec.Body.String())
			}
		}
	}
}
