#!/usr/bin/env bash
# smoke-compose.sh — acceptance check da T-08 (CA-15, fase 00).
# make up → healthz responde 200 em até 60s → make down.
set -u
cd "$(dirname "$0")/.."

echo "smoke: subindo compose dev (make up)..."
if ! make up; then
  echo "FAIL: make up falhou"
  exit 1
fi

ok=0
for _ in $(seq 1 60); do
  if curl -fsS http://localhost:8080/healthz | grep -q '"status":"ok"'; then
    ok=1
    break
  fi
  sleep 1
done

echo "smoke: derrubando compose (make down)..."
make down

if [ "$ok" -eq 1 ]; then
  echo "OK: healthz respondeu 200 dentro de 60s"
  exit 0
fi
echo "FAIL: healthz não respondeu em 60s"
exit 1
