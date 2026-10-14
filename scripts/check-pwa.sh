#!/usr/bin/env bash
# check-pwa.sh — acceptance check (a) da T-14 (CA-08, fase 00).
# Build do PWA: manifest válido + service worker gerado no dist/.
set -u
cd "$(dirname "$0")/.."

fail=0
erro() { echo "FAIL: $1"; fail=1; }

M=frontend/dist/manifest.webmanifest
[ -f "$M" ] || { echo "FAIL: $M não existe"; exit 1; }

grep -q '"name":"Timetrack"' "$M" || erro 'manifest sem "name":"Timetrack"'
grep -q '"display":"standalone"' "$M" || erro 'manifest sem "display":"standalone"'
grep -q '"theme_color":"#0e0e10"' "$M" || erro 'manifest sem theme_color #0e0e10'
grep -q '"sizes":"192x192"' "$M" || erro 'manifest sem ícone 192x192'
grep -q '"sizes":"512x512"' "$M" || erro 'manifest sem ícone 512x512'

[ -f frontend/dist/sw.js ] || erro "service worker (dist/sw.js) não foi gerado"

[ "$fail" -eq 0 ] && echo "OK: manifest PWA válido + service worker gerado"
exit "$fail"
