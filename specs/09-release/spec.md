# spec.md — Fase 09 · Release (RECORTE BETA)

> **Recorte beta (decisão do Diogo, 11/06):** colocar o app EXISTENTE (fases 00–04) em
> produção em `timmingtrack.life`, atrás do Caddy já existente na VPS compartilhada — login
> real, cronômetro e sync entre devices funcionando na internet. **Polish, logo real
> (P-04), backup e ajustes que o uso revelar ficam pro FECHAMENTO da 09**, depois das
> fases 05–08 (esta pasta ganha um anexo quando chegar lá).
> Depende de: fases 00–04 construídas (gate `make check-full` verde) · **P-01** (credencial
> OAuth real) · **P-03** (DNS A `timmingtrack.life` → <IP_DA_VPS>) — `PENDENCIAS-DIOGO.md`.

---

## 1. Mapa pro problema central

| Problema | O que este recorte entrega |
|---|---|
| O app só existe no Mac do Diogo | `https://timmingtrack.life` no ar 24/7, instalável como PWA no iPad/celular/Mac |
| Toggl + Flip Clock no dia a dia | O Diogo começa a usar o Timetrack DE VERDADE (cronômetro + sync) e o uso alimenta as fases 05–08 |

## 2. User stories

- **US-01** — Como Diogo, quero acessar `timmingtrack.life` de qualquer device com meu login
  Google real, pra cronometrar sem depender do Mac de desenvolvimento.
- **US-02** — Como dono da VPS compartilhada, quero o Timetrack 100% isolado (containers
  próprios, zero portas no host), pra produção viva (PrimaOS/brain/jarvis) nunca correr risco.
- **US-03** — Como operador, quero deploy com rollback simples e documentado, pra qualquer
  falha não deixar nem o Timetrack nem os vizinhos quebrados.

## 3. Critérios de aceite (EARS)

### 3.1 Gate (unwanted — o mais importante)

- **CA-01** SE **P-01** (credencial OAuth real no `.env` de prod) OU **P-03** (DNS A
  propagado pra `<IP_DA_VPS>`) não estiverem prontos, o deploy NÃO DEVE prosseguir —
  PARAR e chamar o Diogo. **NUNCA subir produção com Google stubbado ou credencial
  placeholder.**

### 3.2 Isolamento da VPS (ubíquos — constituição §8.2)

- **CA-02** O Timetrack DEVE rodar em containers próprios (`timetrack-app`,
  `timetrack-db`) **sem publicar NENHUMA porta no host** — a única exposição é o vhost no
  Caddy existente (`primaos-caddy-1`, que já detém 80/443).
- **CA-03** O vhost `timmingtrack.life` DEVE entrar por **append** no Caddyfile existente
  (`/opt/primaos/deploy/Caddyfile`), com **backup datado antes** e `caddy validate` antes
  do reload; NENHUMA linha dos vhosts vizinhos muda; NENHUM arquivo do compose do PrimaOS
  é tocado.
- **CA-04** QUANDO o deploy terminar, os vizinhos DEVEM seguir intactos:
  `https://hub.primorlabs.cloud` responde **200** e os containers `primaos-*` seguem
  `Up` **sem restart** (uptime não resetado pelo deploy).

### 3.3 O app no ar (event-driven)

- **CA-05** QUANDO o deploy terminar, `https://timmingtrack.life/healthz` DEVE responder
  **200 com TLS válido** (certificado ACME emitido pelo Caddy).
- **CA-06** QUANDO o container `timetrack-app` sobe, as migrações DEVEM aplicar no boot
  (log `migrações aplicadas`) e os dados DEVEM persistir no volume nomeado `tt-db-data`
  (restart do compose não perde dados).
- **CA-07** O `.env` de produção DEVE ter: credencial Google REAL (P-01) ·
  `TT_TOKEN_KEY` de 64 hex recém-gerada · `COOKIE_SECURE=true` ·
  `OAUTH_REDIRECT_URL=https://timmingtrack.life/auth/google/callback` · senha forte própria
  do Postgres — e NUNCA ser commitado nem logado (constituição §8.7).
- **CA-08** QUANDO um cliente pede `GET /api/v1/ws` em `https://timmingtrack.life` sem
  sessão, a resposta DEVE ser o `401` canônico — prova de que a cadeia
  Caddy → app (inclusive o caminho do WebSocket) atravessou o proxy.

### 3.4 Validação de uso real (manual — Diogo)

- **CA-09** *(manual)* O Diogo DEVE conseguir: logar com a conta Google real → iniciar e
  parar um timer → abrir 2 abas/devices e ver o pause de um abrir a pausa Pomodoro no
  outro (o fluxo central do produto, agora em produção).

### 3.5 Rollback (unwanted)

- **CA-10** SE qualquer smoke falhar, o rollback DEVE deixar a VPS como antes em ≤2
  passos documentados: (1) restaurar o backup do Caddyfile + reload; (2) `docker compose
  down` do Timetrack — vizinhos nunca afetados em nenhum cenário.
- **CA-11** O deploy NÃO DEVE copiar pra VPS: `SECRETS.local.md`, `.env` de dev,
  `node_modules`, `dist` local, artefatos de teste, nem o histórico `.git` (o código vai
  por rsync de working tree limpo).

## 4. Requisitos não-funcionais

- **RNF-01** Zero downtime dos vizinhos (o reload do Caddy é graceful; nada mais é tocado).
- **RNF-02** Build da imagem NA VPS (x86_64) — imagem buildada no Mac (arm64) não roda lá.
- **RNF-03** Janela de exposição mínima: app+db sobem (sem exposição) ANTES do vhost entrar.

## 5. Fora de escopo (fecham a 09 depois das fases 05–08)

- Polish visual, logo/ícone real (P-04), ajustes do que o uso em prod revelar.
- Backup automatizado do Postgres (beta = volume persistente; backup vira CA do fechamento).
- CI/CD, deploy automatizado recorrente (beta = deploy manual documentado).
- Monitoramento/alertas além do healthz.
- P-02 (publicar o app OAuth) — obrigatório só ANTES da fase 06 (Calendar).
