# plan.md — Fase 09 · Release (RECORTE BETA)

> Como executar a `spec.md` desta pasta. Herda o baseline do `SDD.md §7.1/§7.6` e os
> artefatos da fase 00: `deploy/docker-compose.yml`, `deploy/Caddyfile.snippet`,
> `deploy/.env.example`. Credenciais e fatos da VPS em `SECRETS.local.md` (gitignored).

---

## 1. Fatos da VPS (CONFIRMADOS por inspeção read-only em 11/06 — sessão 9)

| Fato | Valor |
|---|---|
| Rede do Caddy | **`primaos_primaos_net`** (bridge; o `primaos-caddy-1` tem alias `caddy` nela) — substitui o placeholder `caddy_net` |
| Caddyfile | `/opt/primaos/deploy/Caddyfile`, bind-mount em `/etc/caddy/Caddyfile` do `primaos-caddy-1` |
| Quem detém 80/443 | `primaos-caddy-1` (NUNCA tocar) |
| Vizinhos vivos | `primaos-app-1`, `primaos-socket-proxy-1` + projetos em `/opt`: brain, caddy, containerd, jarvis, primaos |
| Host | Ubuntu 25.04 x86_64 · 2 vCPU · 7.8 GB · ~90 GB livres · sem Go no host |
| Acesso | `ssh timetrack-vps` (chave dedicada já instalada) |

## 2. Decisões (com razão)

| # | Decisão | Razão |
|---|---|---|
| D-01 | **Build da imagem NA VPS** (`docker compose build`, context `..` do compose da fase 00). Código vai por **rsync do working tree** com excludes (CA-11). | Mac é arm64, VPS é x86_64 — imagem local não roda lá. Sem registry no MVP; sem `.git` na VPS (o repo não tem remote e segredos nunca viajam). |
| D-02 | App em **`/opt/timetrack`** na VPS (`/opt/timetrack/deploy/.env` chmod 600). | Segue o padrão da casa (`/opt/*`). `.env` fica FORA do rsync (criado uma vez, preservado por `--exclude`). |
| D-03 | Rede externa do compose = **`primaos_primaos_net`** (D-01 da spec 00 mantido: `timetrack-db` SÓ na rede `internal`). | O Caddy resolve `timetrack-app:8080` pelo alias de serviço na rede compartilhada; o Postgres fica inalcançável até pros vizinhos. |
| D-04 | Vhost por **append** no Caddyfile do PrimaOS + `caddy validate` + `caddy reload` (exec no container, sem restart). Backup datado antes (`Caddyfile.bak-AAAAMMDD-HHMMSS`). | Reload é graceful (RNF-01); validate barra Caddyfile quebrado ANTES de afetar os vizinhos; o backup é o passo 1 do rollback (CA-10). |
| D-05 | Ordem: **(1)** rsync+build+up (app sem exposição) → **(2)** healthz INTERNO ok → **(3)** vhost+reload → **(4)** smoke externo. | RNF-03: o vhost só entra quando o app já responde; ACME só dispara com DNS propagado (gate CA-01). |
| D-06 | Smoke automatizado: healthz 200 + TLS · `GET /api/v1/ws` sem sessão = 401 canônico (CA-08) · `hub.primorlabs.cloud` 200 · `docker ps` dos vizinhos sem restart. Login real + sync 2 abas = **manual do Diogo** (CA-09). | O login Google real exige a conta do Diogo no browser — inautomatizável por definição. |

## 3. Topologia final (baseline §7.1, agora com nomes reais)

```
internet ──→ primaos-caddy-1 (:80/:443, ACME)            [rede primaos_primaos_net]
               ├── hub.primorlabs.cloud → primaos-app-1   (INTOCADO)
               └── timmingtrack.life       → timetrack-app:8080
                                            └── timetrack-db:5432   [rede internal, isolada]
```

- Caddy v2 faz o upgrade de WebSocket no `reverse_proxy` sem config extra — o
  `/api/v1/ws` da fase 04 atravessa o mesmo vhost.
- Snippet (deploy/Caddyfile.snippet, inalterado):
  `timmingtrack.life { reverse_proxy timetrack-app:8080 }`

## 4. rsync — o que vai e o que NUNCA vai (CA-11)

```
rsync -az --delete \
  --exclude '.git' --exclude 'SECRETS.local.md' --exclude '.env' --exclude '*.local.md' \
  --exclude 'node_modules' --exclude 'frontend/dist' --exclude 'frontend/dev-dist' \
  --exclude 'test-results' --exclude 'playwright-report' --exclude 'deploy/.env' \
  ./  timetrack-vps:/opt/timetrack/
```
O `--exclude deploy/.env` PROTEGE o env de produção criado na T-03 contra `--delete`.

## 5. Armadilhas conhecidas

- **ACME:** o certificado só emite com o DNS A propagado (CA-01 checa `dig` ANTES); se o
  reload acontecer sem DNS, o Caddy fica re-tentando e loga erro — não derruba vizinhos,
  mas o gate existe pra nem chegar lá.
- **`caddy reload` exige o config path:** `docker exec primaos-caddy-1 caddy reload
  --config /etc/caddy/Caddyfile`. Sem `--config` ele procura no cwd e falha.
- **Build na VPS usa o Dockerfile da raiz** (fase 00): multi-stage com build do frontend
  embutido — o `dist/` local NÃO é necessário (e não viaja, CA-11).
- **`docker compose` do Timetrack roda DE `/opt/timetrack/deploy`** — nunca confundir com
  o compose do PrimaOS em `/opt/primaos`.
- **Primeiro boot:** o app espera o Postgres healthy (depends_on da fase 00) e aplica
  migração no boot; `docker compose logs timetrack-app` deve mostrar `migrações aplicadas`
  e `servindo`.

## 6. Rollback (CA-10 — deixar a VPS como antes)

1. `cp /opt/primaos/deploy/Caddyfile.bak-<ts> /opt/primaos/deploy/Caddyfile` +
   `docker exec primaos-caddy-1 caddy reload --config /etc/caddy/Caddyfile`.
2. `docker compose -f /opt/timetrack/deploy/docker-compose.yml down` (volume `tt-db-data`
   sobrevive por padrão; `-v` só se o Diogo mandar zerar).
