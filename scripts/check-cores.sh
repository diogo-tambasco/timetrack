#!/usr/bin/env bash
# check-cores.sh — acceptance check da T-09 (CA-11.1, fase 00).
# tokens.css é a ÚNICA fonte de cor: nenhum hex literal em frontend/src/
# fora de styles/tokens.css.
#
# Allowlist documentada:
#   - frontend/src/styles/tokens.css  → é a própria fonte dos tokens (excluído do grep)
#   - frontend/src/lib/palette.ts     → fase 02: cor de PROJETO é DADO de
#     domínio (a API valida/persiste hex literal #RRGGBB — CA-07), não tema
#     de UI; os valores espelham os --p-*/--accent do tokens.css e o teste
#     lib/palette.spec.ts garante a integridade (divergiu, quebra).
#   - *.spec.ts(x) / *.test.ts(x)     → fixtures de teste carregam cor como
#     dado (objetos Project do schema), não estilizam UI.
# Nada mais é permitido. Precisa de cor nova de TEMA? Vira token no tokens.css.
set -u
cd "$(dirname "$0")/.."

if [ ! -d frontend/src ]; then
  echo "FAIL: frontend/src não existe"
  exit 1
fi

# filtros ANCORADOS no path completo (substring solta deixaria passar
# ex. palette.tsx — finding da verificação adversarial do bloco 3)
matches=$(grep -rnE '#[0-9a-fA-F]{3,8}\b' frontend/src --include='*.ts' --include='*.tsx' --include='*.css' --include='*.html' \
  | grep -vE '^frontend/src/styles/tokens\.css:' \
  | grep -vE '^frontend/src/lib/palette\.ts:' \
  | grep -vE '\.(spec|test)\.(ts|tsx):' || true)

if [ -n "$matches" ]; then
  echo "FAIL: cor hex literal fora do tokens.css:"
  echo "$matches"
  exit 1
fi

echo "OK: nenhuma cor hex fora de styles/tokens.css"
