#!/usr/bin/env bash
# check-fonts.sh — acceptance check da T-10 (CA-13, fase 00).
# Fontes self-hosted: nenhum Google Fonts no bundle, woff2 servido pelo app.
set -u
cd "$(dirname "$0")/.."

fail=0

if [ ! -d frontend/dist ]; then
  echo "FAIL: frontend/dist não existe (rode npm run build antes)"
  exit 1
fi

if grep -rq 'fonts.googleapis.com\|fonts.gstatic.com' frontend/dist; then
  echo "FAIL: bundle referencia Google Fonts:"
  grep -rl 'fonts.googleapis.com\|fonts.gstatic.com' frontend/dist
  fail=1
fi

if ! ls frontend/dist/assets/*.woff2 >/dev/null 2>&1; then
  echo "FAIL: nenhum .woff2 em frontend/dist/assets (Geist deveria ser self-hosted)"
  fail=1
fi

[ "$fail" -eq 0 ] && echo "OK: fontes self-hosted, sem Google Fonts no bundle"
exit "$fail"
