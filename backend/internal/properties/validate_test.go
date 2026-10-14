// validate_test.go — fase 25 T-03 (CA-09): tabela por tipo. Válidos passam;
// inválidos viram validation_error. Não toca DB (unit puro). A mutação que
// remove o ramo `select` (aceitar qualquer string) DEVE quebrar.
package properties

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateValuePorTipo(t *testing.T) {
	opts := []Option{{ID: "op_a", Label: "Lauda"}, {ID: "op_b", Label: "Arkan"}}

	cases := []struct {
		name string
		typ  string
		raw  string
		ok   bool
		// want = jsonb canônico esperado quando ok (vazio = não checa).
		want string
	}{
		// text
		{"text válido", "text", `"oi"`, true, `"oi"`},
		{"text vazio é gravável", "text", `""`, true, `""`},
		{"text >2000 → erro", "text", `"` + strings.Repeat("x", 2001) + `"`, false, ""},
		{"text número → erro", "text", `5`, false, ""},
		// url
		{"url válida", "url", `"https://x.y"`, true, `"https://x.y"`},
		{"url vazia gravável", "url", `""`, true, `""`},
		{"url >2000 → erro", "url", `"` + strings.Repeat("x", 2001) + `"`, false, ""},
		// number
		{"number int", "number", `12`, true, `12`},
		{"number float", "number", `12.5`, true, `12.5`},
		{"number negativo", "number", `-3`, true, `-3`},
		{"number string → erro", "number", `"12"`, false, ""},
		{"number abc → erro", "number", `"abc"`, false, ""},
		{"number bool → erro", "number", `true`, false, ""},
		// date
		{"date válida", "date", `"2026-07-01"`, true, `"2026-07-01"`},
		{"date impossível → erro", "date", `"2026-13-40"`, false, ""},
		{"date texto → erro", "date", `"ontem"`, false, ""},
		{"date número → erro", "date", `20260701`, false, ""},
		// checkbox
		{"checkbox true", "checkbox", `true`, true, `true`},
		{"checkbox false", "checkbox", `false`, true, `false`},
		{"checkbox 1 → erro", "checkbox", `1`, false, ""},
		{"checkbox string → erro", "checkbox", `"true"`, false, ""},
		// select
		{"select id existente", "select", `"op_a"`, true, `"op_a"`},
		{"select id inexistente → erro", "select", `"op_z"`, false, ""},
		{"select número → erro", "select", `1`, false, ""},
		// adiados na v1
		{"person → erro (fatia 25.1)", "person", `"u1"`, false, ""},
		{"multi_select → erro (fatia 25.1)", "multi_select", `["op_a"]`, false, ""},
		{"tipo desconhecido → erro", "foo", `"x"`, false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := validateValue(c.typ, opts, json.RawMessage(c.raw))
			if c.ok {
				if err != nil {
					t.Fatalf("esperava ok, veio erro: %v", err)
				}
				if c.want != "" && string(out) != c.want {
					t.Errorf("jsonb canônico = %s, esperava %s", out, c.want)
				}
				if codeOf(err) != "" {
					t.Errorf("code inesperado: %q", codeOf(err))
				}
				return
			}
			if codeOf(err) != "validation_error" {
				t.Errorf("esperava validation_error, veio %q (out=%s)", codeOf(err), out)
			}
		})
	}
}
