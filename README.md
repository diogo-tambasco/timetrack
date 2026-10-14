# Timetrack

App pessoal de **cronometragem + planejamento de tempo** — consolida Toggl, Google Calendar e
flip clock do iPad em uma ferramenta só. Inspirado no Toggl 2.0, mas mais interativo,
personalizável e com IA.

## O que resolve

1. Trackear tempo **rápido**, separando cliente/projeto.
2. **Sync 2-way com Google Calendar** (fonte da verdade da agenda).
3. **Sync em tempo real entre dispositivos:** pausou num device → pausa em todos → inicia um
   Pomodoro de pausa configurável automaticamente.
4. **Modo Foco** no iPad: flip-clock fullscreen, zero distração.

## Stack

| Camada | Tecnologia |
|---|---|
| Backend | Go (`net/http` + chi) · Postgres · WebSocket |
| Frontend | PWA — React + Vite + TypeScript + Tailwind |
| Auth | Google OAuth |
| Integrações | Google Calendar API (2-way) · skill `/timetrack-task` (Claude) |
| Deploy | Docker atrás de Caddy, VPS própria |

Single-user por enquanto, mas **multi-workspace/multi-user no schema desde o dia 1**.

## Metodologia

Spec-Driven Development + TDD: `spec.md` (EARS) → `plan.md` → testes (red) → código (green).
Ordem de leitura dos docs: `REQUISITOS.md` → `DESIGN.md` → `SDD.md` → `TDD.md` → `specs/`.

## Status

Projeto pessoal, construído com IA: cada feature tem a sua
spec em `specs/` (32 fases), escrita antes do código.

## Rodar localmente

Precisa de Docker, Go e Node. Copie `.env.example` para `.env` e preencha.

```bash
make up           # sobe Postgres + app em Docker
make dev          # backend (:8080) + frontend (:5173) para desenvolver
make check        # testes, build e checagens (inclui varredura de segredos)
```

Deploy: Docker atrás de Caddy numa VPS (ver `deploy/` e `specs/09-release/`).

## Licença

[MIT](LICENSE) © Diogo Tambasco
