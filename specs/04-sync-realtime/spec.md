# spec.md — Fase 04 · Sync em tempo real

> O timer reflete em **todos os dispositivos do usuário** em tempo real, via WebSocket:
> pausei no celular → o iPad e o Mac pausam **e abrem a pausa Pomodoro** (problema central
> nº3 do `REQUISITOS.md §2`). O servidor segue sendo a única fonte da verdade (fase 03);
> o WebSocket é só o **meio de entrega** do mesmo estado canônico — **downstream-only**.
> Depende de: fase 03 (`timer.Service` + hook `OnChange` + `TimerState` canônico).
>
> **Decisões do Diogo (11/06):** broadcast **só do timer** (entries/settings ficam fora) ·
> queda do WS = **reconnect + re-sync** (sem polling novo) · **nenhum indicador de conexão
> na UI** (o comportamento é o único feedback) · **deploy em produção ANTECIPADO logo após a
> 04** (decisão da noite de 11/06, supersede o "manter ordem" da manhã): fase 09 parcial =
> P-01 + P-03 + deploy do que existe; fases 05–08 seguem depois com o app no ar.

---

## 1. Mapa pro problema central (`REQUISITOS.md §2`)

| Problema central | O que esta fase entrega |
|---|---|
| 3. Pausei no celular → **pausa no iPad e no Mac** e **inicia o Pomodoro configurável** | Broadcast do `TimerState` por usuário: toda mutação de timer chega a todas as conexões do usuário; pause entrega `mode='break'` e o BreakOverlay abre em todos os devices |
| 1. Trackear rápido | Start/stop/continue feitos em qualquer device aparecem nos outros sem reload |

## 2. User stories

- **US-01** — Como usuário, quero **pausar num device e ver todos os outros pausarem e
  abrirem a pausa Pomodoro**, pra descansar sem tocar em mais nada. (Núcleo do produto.)
- **US-02** — Como usuário, quero que start/stop/resume/continue feitos em qualquer device
  apareçam nos outros em tempo real, pra barra global ser confiável onde quer que eu olhe.
- **US-03** — Como usuário, quero que o app **se recupere sozinho** quando a conexão cair
  (iPad dormiu, rede oscilou), voltando ao estado certo sem reload manual.
- **US-04** — Como usuário com 2 devices abertos, quero que as transições automáticas
  (countdown zerou, foco zerou, ciclo pomodoro) aconteçam **uma vez só**, sem entry duplicada
  nem erro na tela.

## 3. Critérios de aceite (EARS) — cada CA vira exatamente 1 teste

### 3.1 Invariantes (ubíquos)

- **CA-01** *(ubíquo)* O hub DEVE entregar eventos **apenas às conexões do próprio usuário**:
  com usuários A e B conectados, uma mutação de timer de A NUNCA chega a nenhum socket de B.
  *(vazamento entre usuários = bug crítico)*
- **CA-02** *(ubíquo)* O payload de todo evento de timer DEVE ser o **mesmo objeto canônico**
  da resposta do `GET /api/v1/timer` — `{state, server_time}` — com `server_time` igual ao
  relógio **da mutação** no servidor (não ao instante do broadcast); o cliente o aplica pelo
  mesmo caminho das respostas REST.
- **CA-03** *(ubíquo)* O WebSocket é **unidirecional servidor→cliente**: qualquer mensagem
  enviada pelo cliente DEVE ser ignorada sem efeito e sem derrubar a conexão — toda mutação
  continua passando pelas rotas REST da fase 03 (com toda a cadeia de validação).
- **CA-04** *(ubíquo)* Nenhuma rota REST da fase 03 muda: requisição e resposta de
  `GET /timer`, `start`, `stop`, `pause`, `resume`, `break/skip`, entries e settings
  permanecem byte-compatíveis. A suíte da fase 03 segue verde **sem edição de asserções**;
  as ÚNICAS edições sancionadas (pelo RF-05/D-04 e pelo D-07) são mecânicas: o ajuste de
  assinatura no teste do hook (`timer/service_test.go`) e, se necessário, stub de
  `WebSocket` no **setup** do `timerStore.spec.ts` da 03 — nenhuma asserção muda.

### 3.2 Backend — conexão e broadcast (event-driven)

- **CA-05** QUANDO `GET /api/v1/ws` chega **sem cookie de sessão válido**, o sistema DEVE
  recusar com `401 {"error":{"code":"unauthorized",...}}` antes de qualquer upgrade.
- **CA-06** QUANDO uma mutação de timer é commitada (start, stop, pause, resume, skip,
  continue — e o stop implícito da troca do CA-05 da fase 03 conta como parte da mesma
  mutação), TODAS as conexões abertas do usuário DEVEM receber **exatamente 1 evento** com o
  estado novo (pause → `state.mode='break'`; stop → `state: null`).
- **CA-07** O broadcast obedece **evento ⟺ commit**: NENHUM evento sem transação commitada
  (validação, rollback e conflito sem efeito não emitem), e TODA transação commitada emite —
  **mesmo quando a resposta HTTP é de erro**. Caso real herdado da 03: o resume de pausa
  órfã (`resume_entry_id` NULL — defesa em profundidade) commita a remoção do estado, emite
  o evento `state: null` E responde `409 no_active_break`; os outros devices precisam saber
  que a pausa morreu.
- **CA-08** QUANDO o usuário tem múltiplas conexões (devices e/ou abas), TODAS DEVEM receber
  cada evento — inclusive a conexão do device que **originou** a mutação.
- **CA-09** SE uma conexão não drenar os eventos (consumidor lento/travado), o hub DEVE
  **fechar essa conexão** sem bloquear o broadcast nem atrasar as demais conexões; o cliente
  recupera o estado pela reconexão (CA-14).
- **CA-10** QUANDO uma conexão é encerrada (close do cliente, erro, timeout de ping), o hub
  DEVE removê-la do registro — broadcasts seguintes não tentam entregá-la e não há vazamento
  de conexões acumuladas.

### 3.3 Frontend — aplicação dos eventos (event-driven)

- **CA-11** QUANDO um evento de timer chega pelo WS, o cliente DEVE aplicá-lo via
  `timerStore.apply()` — herdando o anti-stale da fase 03: evento com `server_time` anterior
  ao âncora atual é **descartado** (ordem WS × REST nunca regride o estado).
- **CA-12** QUANDO o pause acontece em **outro** device, o device atual DEVE trocar pro modo
  descanso (BreakOverlay) exibindo o countdown da pausa, **sem reload e sem ação local**.
  *(comportamento central do produto)*
- **CA-13** QUANDO start/stop/resume/continue acontecem em outro device, a barra global do
  device atual DEVE refletir o estado novo (rodando com descrição/projeto, ou ociosa).
- **CA-14** QUANDO a conexão WS abre (primeira conexão **ou** reconexão), o cliente DEVE
  re-sincronizar com `GET /api/v1/timer` — eventos perdidos durante a queda nunca deixam
  estado fantasma.

### 3.4 Resiliência (state-driven)

- **CA-15** ENQUANTO o WS estiver caído, o app DEVE permanecer 100% funcional via REST
  (tick local, mutações, re-sync em `visibilitychange`/focus/60s da fase 03) — a degradação
  é invisível além da perda do tempo-real.
- **CA-16** ENQUANTO o WS não conectar, o cliente DEVE tentar reconectar com **backoff
  exponencial com teto** (sem tempestade de reconexão) e NÃO DEVE criar nenhuma requisição
  periódica nova além das já existentes na fase 03 (sem polling).

### 3.5 Erros e proteções (unwanted)

- **CA-17** SE uma transição automática dispara com 2+ devices abertos (countdown/foco/pausa
  de ciclo zerando em todos ao mesmo tempo), o resultado DEVE ser **exatamente 1 transição**
  no servidor: 1 entry fechada (ou 1 resume), os devices "perdedores" recebem o conflito
  (409), re-sincronizam sozinhos e NENHUM erro aparece pro usuário. *(Prova em 2 camadas: o
  mecanismo da corrida é DETERMINÍSTICO no backend — 2 resumes concorrentes de pausa
  pomodoro expirada, T-05; o E2E prova o resultado fim a fim no braço countdown — T-10. O
  braço do ciclo é inviável em E2E: foco mínimo 15min, pausa mínima 1min.)*
- **CA-18** SE o cliente receber mensagem malformada ou de tipo desconhecido, DEVE ignorá-la
  sem quebrar a conexão nem o estado (forward-compat com tipos de evento futuros).
- **CA-19** QUANDO o runtime do timer é desmontado (teardown do `attachTimerSync`), a conexão
  WS DEVE ser fechada e o agendamento de reconexão cancelado — sem socket nem timer zumbi.

### 3.6 E2E (Playwright — 2 contextos = 2 "devices", fluxo nº2 do TDD.md §4)

- **CA-20** QUANDO o device A pausa, o device B DEVE pausar **e** exibir o BreakOverlay com a
  contagem regressiva da pausa configurada — sem reload, sem interação no B.
- **CA-21** QUANDO o device A inicia um timer com projeto, o device B DEVE exibir a barra
  rodando com a mesma descrição e o chip do projeto em até **3s** (tolerância de ambiente de
  teste; a meta de implementação é < 1s — RNF-02).

## 4. Requisitos funcionais (resumo)

- **RF-01** Hub de broadcast **por usuário** em `internal/realtime`, desacoplado do
  transporte (testável com transporte fake — TDD.md §3).
- **RF-02** Endpoint `GET /api/v1/ws` autenticado pela mesma sessão por cookie da fase 01.
- **RF-03** Evento com envelope versionável: `{"type":"timer","payload":{state,server_time}}`
  — `payload` idêntico ao corpo do `GET /timer`. Tipos novos (entries, settings) ficam pra
  fases futuras; o envelope já permite.
- **RF-04** Cliente WS no PWA integrado ao `timerStore` (`apply()` como único ponto de
  entrada de estado), com reconexão automática e teardown limpo.
- **RF-05** Hook `OnChange` do `timer.Service` estendido **aditivamente** com o instante da
  mutação (`at time.Time`) — extensão sancionada; ver plan.md D-04.

## 5. Requisitos não-funcionais

- **RNF-01 Leveza:** exatamente **1 dependência Go nova** (lib WebSocket — decisão explícita
  no plan.md §4); **zero** dependência JS nova (WebSocket nativo do browser).
- **RNF-02 Latência:** mutação refletida nos outros devices em **< 1s** em rede local (meta
  de implementação; o E2E do CA-21 usa 3s de tolerância — números intencionais e distintos,
  mesmo padrão do RNF-02 da fase 03).
- **RNF-03 Sem polling:** nenhuma requisição periódica nova; o re-sync continua sendo só o
  da fase 03 + o evento de reconexão (CA-14).
- **RNF-04 Robustez do hub:** um consumidor lento não degrada os demais (CA-09) e o hub não
  vaza conexões (CA-10).
- **RNF-05 Zero refatoração da fase 03:** rotas, payloads e comportamento REST intactos
  (CA-04); a única mudança em código da 03 é a extensão aditiva do hook (RF-05).
- **RNF-06 Pronto pras fases 05/06+:** o Modo Foco (05) consome o mesmo `timerStore` já
  sincronizado — nada a fazer lá; o envelope do RF-03 comporta tipos novos sem quebrar
  clientes antigos (CA-18).

## 6. Fora de escopo (fronteiras explícitas)

- **Broadcast de entries e settings** (lista Hoje viva entre devices, settings ao vivo) —
  decisão do Diogo (11/06): só timer na v1 do sync. O envelope `type` já reserva o caminho.
- **Indicador de conexão na UI** (dot/toast) — decisão do Diogo (11/06): sync invisível.
- **Persistência local do estado do timer e cache de `/api` no Service Worker** —
  **PROIBIDOS nesta fase** (aviso 1 do handoff da sessão 7: o `offline.spec.ts` depende de
  offline → estado nulo → IdleBar; introduzir persistência exigiria rever aquele contrato
  em spec própria).
- **Broadcast por workspace / presença / multiplayer** — fora do MVP (constituição §8.8).
- **Notificações push / som** ao pausar em outro device — fora (candidato pra 05/09).
- **Polling de fallback** quando o WS cai — decisão do Diogo (11/06): reconnect + re-sync
  bastam; o REST da fase 03 já cobre o intervalo.
- **Modo Foco fullscreen** → fase 05 (consome esta fase pronta).
