// validate.go — fase 25 T-03 (CA-09): validação por tipo do value de uma
// propriedade. switch por def.type (D-07, na mão com stdlib — leveza RNF-01).
// O value chega como json.RawMessage e é decodificado tipado conforme o type;
// devolve o jsonb normalizado (canônico) ou *domain.Error (validation_error).
package properties

import (
	"bytes"
	"encoding/json"
	"math"
	"time"

	"timetrack.live/backend/internal/domain"
)

// Option é uma opção de um select (id estável + label + color opcional).
// Vive aqui porque a validação de `select` é quem consulta o id (D-08).
type Option struct {
	ID    string  `json:"id"`
	Label string  `json:"label"`
	Color *string `json:"color"`
}

const maxTextLen = 2000

// validateValue valida o value NÃO-null contra o type da def (CA-09). O caso
// `value:null` (clear) é tratado ANTES de chamar isto (CA-09.1, no service/
// handler) — aqui o raw nunca é "null". Retorna o jsonb canônico ou erro.
func validateValue(typ string, options []Option, raw json.RawMessage) (json.RawMessage, error) {
	switch typ {
	case "text", "url":
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, domain.Validation("valor inválido para o tipo da propriedade")
		}
		if len(s) > maxTextLen {
			return nil, domain.Validation("valor inválido para o tipo da propriedade")
		}
		return canonical(s)

	case "number":
		// json.Number pra distinguir número de string numérica ("12" → erro).
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		var tok any
		if err := dec.Decode(&tok); err != nil {
			return nil, domain.Validation("valor inválido para o tipo da propriedade")
		}
		num, ok := tok.(json.Number)
		if !ok {
			return nil, domain.Validation("valor inválido para o tipo da propriedade")
		}
		f, err := num.Float64()
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return nil, domain.Validation("valor inválido para o tipo da propriedade")
		}
		return canonical(f)

	case "date":
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, domain.Validation("valor inválido para o tipo da propriedade")
		}
		if _, err := time.Parse("2006-01-02", s); err != nil {
			return nil, domain.Validation("valor inválido para o tipo da propriedade")
		}
		return canonical(s)

	case "checkbox":
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, domain.Validation("valor inválido para o tipo da propriedade")
		}
		return canonical(b)

	case "select":
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, domain.Validation("valor inválido para o tipo da propriedade")
		}
		for _, o := range options {
			if o.ID == s {
				return canonical(s)
			}
		}
		return nil, domain.Validation("valor inválido para o tipo da propriedade")

	default:
		// person/multi_select (fatia 25.1) e qualquer tipo desconhecido caem aqui
		// na v1 (CA-02 já barra a CRIAÇÃO da def com esses tipos).
		return nil, domain.Validation("valor inválido para o tipo da propriedade")
	}
}

// canonical re-serializa o valor já tipado pro jsonb canônico.
func canonical(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, domain.Validation("valor inválido para o tipo da propriedade")
	}
	return json.RawMessage(b), nil
}
