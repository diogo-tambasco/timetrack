#!/usr/bin/env bash
# check-segredos.sh — acceptance check da T-01 (CA-17, fase 00).
# Verifica que segredos estão fora do git e que o .env.example só tem placeholders.
set -u
cd "$(dirname "$0")/.."

fail=0
erro() { echo "FAIL: $1"; fail=1; }

# 1) Arquivos de segredo DEVEM ser ignorados pelo git
for f in .env .env.local SECRETS.local.md; do
  git check-ignore -q "$f" || erro "$f não está coberto pelo .gitignore"
done

# 2) .env.example DEVE existir e NÃO ser ignorado (vai pro git)
if [ ! -f .env.example ]; then
  erro ".env.example não existe"
else
  git check-ignore -q .env.example && erro ".env.example está sendo ignorado pelo git (falta '!.env.example')"

  # 3) .env.example DEVE conter as variáveis necessárias da fase
  for var in DATABASE_URL ADDR; do
    grep -q "^${var}=" .env.example || erro ".env.example não documenta ${var}"
  done

  # 4) Só placeholders — nenhum valor com cara de segredo real.
  # Allowlist documentada: valores aceitos são placeholders óbvios (user:pass@, localhost,
  # 'troque-me', vazio ou comentário). Heurística: rejeita strings longas de alta entropia
  # (40+ chars alfanuméricos contíguos), padrão de chaves/API keys reais.
  if grep -qE '=[A-Za-z0-9+/_-]{40,}' .env.example; then
    erro ".env.example contém valor com cara de segredo real (string de alta entropia)"
  fi
fi

if [ "$fail" -eq 0 ]; then
  echo "OK: segredos fora do git, .env.example só com placeholders"
fi
exit "$fail"
