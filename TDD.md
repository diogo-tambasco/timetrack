# TDD.md — Test-Driven Development · Timetrack

> **Regra de ouro:** nenhuma linha de código de produção sem um teste que **falhou antes**.
> red → green → refactor, por tarefa. **Um teste para cada critério de aceite (EARS) da spec.**
> Backend: **`go test`** (stdlib `testing` + `testify` se ajudar). Frontend: **Vitest** (unit) +
> **Playwright** (E2E). Pareado com SDD (`SDD.md`).

---

## 1. O ciclo (obrigatório por tarefa)

```
RED      escreve o teste a partir do critério de aceite da spec  →  RODA  →  CONFIRMA QUE FALHA
GREEN    escreve o MÍNIMO de código pra passar                   →  RODA  →  verde
REFACTOR limpa com os testes como rede de segurança              →  RODA  →  segue verde
COMMIT   commit pequeno (teste + código), mensagem PT-BR
```

### Anti-cheat (crítico p/ build autônomo com Fable 5)
- **Confirmar o RED antes do GREEN.** Se o teste passa sem o código novo, ele não exercita nada →
  refazer. (Pular essa etapa é o erro nº1 de TDD com agente.)
- Proibido: código antes do teste · ajustar o teste pra passar em vez de ajustar o código ·
  `t.Skip`/comentar teste pra "ficar verde" · asserts vazios/tautológicos.
- Se uma tarefa não dá pra testar, está mal definida na spec → **corrige a spec primeiro**.

## 2. Pirâmide de testes (muitos embaixo, poucos em cima)

| Camada | Ferramenta | Testa |
|---|---|---|
| **Unit (Go)** | `go test` | lógica de timer (count up/down), regras de Pomodoro, cálculo de duração, validações de domínio |
| **Domínio / DB** | `go test` + Postgres de teste | isolamento por `workspace_id`, visibilidade de projeto, roles, CRUD |
| **Realtime** | `go test` (hub fake) | broadcast de start/stop/**pause→pomodoro** por user/workspace, reconciliação |
| **API / integração** | `go test` (httptest) | auth exigida, token `tt_` com escopo, contratos REST, erros |
| **Calendar** | `go test` (Google API **stubbada**) | sync 2-way (entry↔evento), expiração/reconexão de token |
| **Unit (front)** | Vitest | store do timer, formatação do flip-clock, lógica de Pomodoro na UI |
| **E2E (front)** | Playwright | os fluxos centrais (ver §4) — inclusive sync entre 2 "devices" (2 contextos) |

## 3. Serviços externos nos testes = SEMPRE fake/stub

- **Nunca** bater na **VPS real**, no **Google real** nem no **Postgres de produção** em teste.
- Google Calendar / OAuth: cliente **stubbado** com fixtures JSON (incluindo token expirado).
- WebSocket: hub com transporte **fake** em memória pra testar broadcast determinístico.
- Postgres: instância de teste isolada (container efêmero / schema de teste), nunca a de prod.

## 4. Mapa critério central → teste E2E

| # | Critério (REQUISITOS) | Teste E2E |
|---|---|---|
| 1 | trackear rápido, separar cliente/projeto | cria projeto → inicia timer → para → entrada aparece com projeto certo |
| 2 | sync tempo real entre devices | 2 contextos: pausa no device A → device B pausa **e** abre Pomodoro |
| 3 | Modo Foco (flip-clock) | abre Modo Foco no "iPad" → dígitos grandes + projeto → pause abre Pomodoro configurado |
| 4 | Google Calendar 2-way | (stub) evento do GCal vira bloco planejado · entry cronometrada reflete no GCal |
| 5 | relatórios (4 ângulos) | semana com entradas → relatório mostra tempo/projeto, foco×fragmentação, meta×real, plan×real |
| 6 | conector Claude | `/timetrack-task new` → escolhe projeto+nome → task criada **e timer rodando** (via API token) |

## 5. Por fase (foco de teste)

- **00 Esqueleto:** boot do server, healthcheck, migrate sobe, PWA carrega, deploy mínimo.
- **01 Auth:** OAuth Google (stub), sessão, criação de Workspace/Owner, isolamento.
- **02 Core:** CRUD projetos/tasks, visibilidade privado×compartilhado, roles, filtro por workspace.
- **03 Cronômetro:** count up/down, Pomodoro configurável, entrada manual, cálculo de duração.
- **04 Sync:** broadcast por user/workspace, **pause→pomodoro em todos**, reconciliação ao reconectar.
- **05 Modo Foco:** render flip-clock, Wake Lock (mock), pause→Pomodoro.
- **06 Calendar:** sync 2-way com Google stubbado, expiração/reconexão de token.
- **07 Relatórios:** agregações corretas dos 4 ângulos, metas e desvios.
- **08 Conector:** API token escopo, `/timetrack-task` cria+inicia, negação sem token.
- **09 Release:** smoke do deploy atrás do Caddy, vizinhos intactos.

## 6. Disciplina

- **Suíte verde ao fim de cada fase** — pré-requisito p/ avançar.
- Regressão: bug → primeiro um teste que reproduz (red), depois o fix (green).
- Cobertura não é dogma, mas **caminhos críticos** (isolamento multi-tenant, sync de timer,
  pause→pomodoro, auth/token, sync de calendário) **têm que** estar cobertos.
- Rodar a suíte é parte do "implement" de toda tarefa, não opcional.
