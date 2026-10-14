package notes

import (
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// pgxValidUUID é um alias fino sobre uuid.Validate (centraliza o guard de uuid
// malformado → erro de domínio nos callers).
func pgxValidUUID(s string) error { return uuid.Validate(s) }

// markers de markdown removidos pelo strip do preview (D-05) — espelha o md()
// do mockup, sem renderizar HTML: títulos, ênfase, code, listas, citação,
// link [t](u) → t, code-fence, headers de tabela.
var (
	reHeading   = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	reListMark  = regexp.MustCompile(`(?m)^\s*([-*+]|\d+\.)\s+(\[[ xX]\]\s+)?`)
	reBlockq    = regexp.MustCompile(`(?m)^\s*>\s?`)
	reLink      = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	reEmphasis  = regexp.MustCompile("(\\*\\*|__|\\*|_|`)")
	reCodeFence = regexp.MustCompile("(?m)^```.*$")
	reHR        = regexp.MustCompile(`(?m)^\s*([-*_]\s*){3,}$`)
	reSpaces    = regexp.MustCompile(`\s+`)
)

// mdStrip remove a marcação markdown do body e devolve os primeiros ≤140
// caracteres de texto puro (D-05/CA-03). NÃO renderiza HTML — é um strip de
// texto pro preview da lista.
func mdStrip(body string) string {
	s := body
	s = reCodeFence.ReplaceAllString(s, " ")
	s = reHR.ReplaceAllString(s, " ")
	s = reHeading.ReplaceAllString(s, "")
	s = reListMark.ReplaceAllString(s, "")
	s = reBlockq.ReplaceAllString(s, "")
	s = reLink.ReplaceAllString(s, "$1")
	s = reEmphasis.ReplaceAllString(s, "")
	s = reSpaces.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	// corta em 140 runas (não bytes), preservando UTF-8.
	runes := []rune(s)
	if len(runes) > 140 {
		runes = runes[:140]
	}
	return string(runes)
}
