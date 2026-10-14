# tasks.md — Fase 09 · Release (RECORTE BETA)

> Tarefas atômicas, em ordem. Deploy é infra: o "teste" de cada tarefa é o acceptance
> check verificável (smoke), não go test. **T-01 é GATE: sem P-01 + P-03, PARAR e chamar
> o Diogo** — nenhuma tarefa seguinte roda. Commits: T-02 commita no repo; T-03+ são
> ações na VPS (registradas no handoff do CLAUDE.md).

---

- **T-01 — GATE de pré-requisitos (CA-01, CA-07)**
  - **Objetivo:** confirmar que dá pra ir pra produção DE VERDADE.
  - **Checks:** `dig +short timmingtrack.life A` → `<IP_DA_VPS>` (P-03) · credencial
    OAuth real disponível (P-01, via Diogo/SECRETS.local.md atualizado) · montar o
    `.env` de produção local-temporário a partir de `deploy/.env.example` com:
    GOOGLE_CLIENT_ID/SECRET reais · `TT_TOKEN_KEY=$(openssl rand -hex 32)` ·
    `POSTGRES_PASSWORD` forte gerada · `DATABASE_URL` com a mesma senha ·
    `OAUTH_REDIRECT_URL=https://timmingtrack.life/auth/google/callback` ·
    `COOKIE_SECURE=true`.
  - **Acceptance check:** os 2 pré-requisitos confirmados E `.env` completo (nenhum
    `troque-me` sobrando). **FALTOU QUALQUER UM → PARAR, briefing pro Diogo, fim.**

- **T-02 — Compose de produção com a rede REAL (CA-02)**
  - **Objetivo:** trocar o placeholder `caddy_net` por `primaos_primaos_net` (fato
    confirmado — plan §1) no `deploy/docker-compose.yml`.
  - **Acceptance check:** `docker compose -f deploy/docker-compose.yml config` valida
    local (com .env dummy) · nenhum `ports:` em nenhum serviço · commit no repo.

- **T-03 — Código + env na VPS, build e subida (CA-06, CA-11, RNF-02/03)**
  - **Objetivo:** rsync com os excludes do plan §4 pra `/opt/timetrack` · criar
    `/opt/timetrack/deploy/.env` (chmod 600) com o conteúdo do T-01 · `docker compose
    build` e `up -d` NA VPS.
  - **Acceptance check:** `docker compose ps` = app+db Up (db healthy) · log do app tem
    `migrações aplicadas` e `servindo` · ZERO portas publicadas (`docker ps` sem
    `0.0.0.0` nos containers timetrack) · healthz INTERNO 200 (`docker exec` curl da
    própria rede) · `SECRETS.local.md`/`.git` NÃO existem em `/opt/timetrack`.

- **T-04 — Vhost no Caddy existente (CA-03, CA-05)**
  - **Objetivo:** backup datado do `/opt/primaos/deploy/Caddyfile` · append do
    `deploy/Caddyfile.snippet` · `caddy validate` · `caddy reload --config
    /etc/caddy/Caddyfile` no container `primaos-caddy-1`.
  - **Acceptance check:** validate OK ANTES do reload · diff do Caddyfile = só o bloco
    novo no fim · `https://timmingtrack.life/healthz` responde 200 com certificado válido
    (ACME emitido) em ≤2 min.

- **T-05 — Smoke externo + vizinhos (CA-04, CA-08, RNF-01)**
  - **Objetivo:** provar o app no ar E a VPS intacta.
  - **Acceptance check:** `curl -s https://timmingtrack.life/healthz` → 200 ·
    `curl -s https://timmingtrack.life/api/v1/ws` → `401 {"error":{"code":"unauthorized"}}`
    (cadeia Caddy→app incluindo o caminho do WS) · `https://timmingtrack.life/` serve o
    shell (200, html) · `https://hub.primorlabs.cloud` → 200 · `docker ps` mostra os 3
    `primaos-*` Up com uptime ANTERIOR ao deploy (sem restart).

- **T-06 — Validação manual do Diogo (CA-09) + fechamento**
  - **Objetivo:** o smoke que só o dono da conta Google faz.
  - **Checks do Diogo:** login real em `https://timmingtrack.life` · iniciar/parar timer ·
    2 abas: pause numa abre a pausa Pomodoro na outra · instalar o PWA (opcional).
  - **Acceptance check:** OK do Diogo · handoff no CLAUDE.md ("app EM PRODUÇÃO") ·
    INDEX marcando o recorte beta da 09 · KICKOFF da próxima sessão (specs da fase 05).

---

## Matriz CA → tarefa

| CA | Tarefa | | CA | Tarefa |
|---|---|---|---|---|
| CA-01 | T-01 (gate) | | CA-07 | T-01 + T-03 |
| CA-02 | T-02 + T-03 | | CA-08 | T-05 |
| CA-03 | T-04 | | CA-09 | T-06 (manual) |
| CA-04 | T-05 | | CA-10 | plan §6 (procedimento; exercitado só se falhar) |
| CA-05 | T-04 + T-05 | | CA-11 | T-03 |
| CA-06 | T-03 | | | |
