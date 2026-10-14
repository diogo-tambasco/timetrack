# spec.md — Fase 11 · Calendário 2.0 (grade de horários estilo Google Calendar)

> Fonte de verdade da fase. Redesenha a página `/calendario` (hoje uma lista agrupada por
> dia, fase 06) numa **grade de horários** estilo Google Calendar, com **3 views** e
> **personalização persistida**.
> **Decisões travadas com o Diogo (HITL, 15/06/2026):**
> 1. **Views:** Semana + Dia + Mês.
> 2. **Grade:** faixa default **6h–23h**, slots de **1h**, **linha do "agora"** + **auto-scroll**
>    pro horário atual, **semana começa na segunda** (consistente com os relatórios).
> 3. **Cor dos eventos:** **accent do app** (NÃO a cor do Google — o `colorId` segue descartado
>    no `internal/calendar/client.go`; a ETAPA 1.5 do kickoff fica fora desta fase).
> 4. **Persistência das preferências:** **backend** (sincroniza entre dispositivos) — migração
>    nova + endpoints, espelhando o padrão `pomodoro_settings` da fase 03.

---

## 1. Objetivo

Transformar `/calendario` numa visão de agenda de verdade: uma **grade hora × dia** (semana e
dia) e uma **grade de células** (mês), onde os eventos do Google Calendar aparecem
**posicionados pelo horário** (topo/altura proporcionais ao tempo), com navegação por período,
troca de view, linha do "agora", e preferências (view padrão, faixa de horas, densidade) que
**seguem o usuário entre dispositivos**.

**A camada de dados da fase 06 é reusada inteira** (`GET /api/v1/calendar/{status,events}` +
os estados Conectar/Erro/Reconectar). Esta fase é **frontend** (a grade, o posicionamento por
tempo, a troca de views, o popover) + **um backend mínimo** novo: persistência das preferências
de calendário por usuário.

GCal é a fonte da verdade da agenda (REQUISITOS §1); como na 06, o Timetrack **lê** — esta fase
**não escreve** no Google (sem criar/editar/mover evento).

## 2. User stories

- **US-01** — Como usuário, quero ver minha agenda numa **grade de horários** (igual ao Google
  Calendar), pra bater o olho e entender meu dia/semana sem ler uma lista.
- **US-02** — Como usuário, quero **trocar entre Dia, Semana e Mês**, pra zoom-in num dia ou
  visão ampla do mês.
- **US-03** — Como usuário, quero uma **linha do "agora"** e o calendário já **rolado pro
  horário atual**, pra achar o "agora" sem rolar.
- **US-04** — Como usuário, quero **clicar num evento** e ver o detalhe num popover com link
  pra **abrir no Google**, pra agir sobre o evento.
- **US-05** — Como usuário, quero **escolher minha view padrão, minha faixa de horas e a
  densidade**, e que isso **me siga no Mac, iPad e celular**, pra o app ser do meu jeito em
  todo lugar.

## 3. Critérios de aceite (EARS — cada um vira teste)

### Preferências de calendário (backend, por usuário)

- **CA-01** — `GET /api/v1/settings/calendar` DEVE responder as preferências do usuário da
  sessão; **sem linha persistida**, DEVE responder os **defaults**: `default_view:"week"`,
  `hour_start:6`, `hour_end:23`, `density:"comfortable"`. Sem sessão → **401** canônico.
- **CA-02** — `PUT /api/v1/settings/calendar` com corpo válido DEVE fazer **upsert por
  usuário** e responder o estado salvo (200). Sem sessão → 401.
- **CA-03** (validação) — `PUT` DEVE rejeitar com **400 `validation_error`** (sem persistir):
  `default_view` fora de `{day,week,month}`; `density` fora de `{comfortable,compact}`;
  `hour_start` fora de `[0,23]`; `hour_end` fora de `[1,24]`; ou `hour_end <= hour_start`.
  Os `CHECK` da migração são o backstop.
- **CA-04** (isolamento) — a leitura/escrita DEVE usar exclusivamente o `user_id` da sessão; a
  linha de outro usuário NUNCA é lida nem sobrescrita. (Constituição §8.4.)

### Reuso da camada de dados (fase 06) — janelas por view

- **CA-05** (janela da semana) — na view **Semana**, a página DEVE buscar eventos com
  `from = segunda` e `to = domingo` (a `weekRange` atual: segunda→domingo local), reaproveitando
  `GET /calendar/events?from&to&tz` com o fuso do navegador.
- **CA-06** (janela do dia) — na view **Dia**, a página DEVE buscar com `from = to = o dia
  exibido` (a janela do backend é `[dia 00:00, dia+1 00:00)`, cobrindo o dia inteiro).
- **CA-07** (janela do mês) — na view **Mês**, a página DEVE buscar com `from = primeira
  célula visível` e `to = última célula visível` da grade mensal (a grade cobre semanas
  parciais do mês anterior/seguinte — ver CA-18), num único request.
- **CA-08** (só conectado) — em qualquer view, eventos só são buscados quando
  `status.connected = true` (preserva o gate da 06, evita o 409 `calendar_not_connected`).

### Posicionamento por tempo (o coração — funções puras testáveis)

- **CA-09** (topo/altura) — um evento **com horário** (`all_day:false`) DEVE ser posicionado na
  coluna do seu dia local com `top` proporcional ao **horário de início local** e `height`
  proporcional à **duração** (em coordenada de relógio de parede da grade): `top = (startH +
  startMin/60) × rowHeight` e `height = (endLocal − startLocal em horas) × rowHeight`, medido na
  **mesma data local** do segmento. A grade renderiza as **24 horas** (0–24) — nenhum evento
  fica invisível (a faixa de horas é só viewport/scroll, CA-15).
- **CA-10** (altura mínima) — eventos muito curtos DEVEM ter uma **altura mínima legível**
  (não colapsar a zero) pra o título caber e o clique funcionar.
- **CA-11** (cruza meia-noite) — um evento que cruza a meia-noite (ex.: 23:00→01:00 do dia
  seguinte) DEVE ser **dividido em segmentos por dia**, cada segmento **clipado** a
  `[00:00, 24:00)` na coluna do seu dia (dia 1: 23:00→24:00; dia 2: 00:00→01:00). O mesmo vale
  pra eventos de vários dias com horário.
- **CA-11.1** (fim exatamente na meia-noite) — um segmento cuja duração de parede após o clip
  é **zero** (`endMin == startMin` — caso de um evento que termina exatamente em `00:00`, ex.:
  10:00→00:00) DEVE ser **DESCARTADO** antes de aplicar a altura mínima (CA-10). Um evento
  10:00→00:00 (fim no dia seguinte) gera **um único** segmento 10:00–24:00 no dia do início,
  **nenhum** bloco-fantasma no dia seguinte. (Sem isto, a altura mínima do CA-10 transforma o
  segmento de 0min num bloquinho clicável num dia em que o evento não acontece.)
- **CA-12** (sobreposição → colunas) — eventos **com horário que se sobrepõem no tempo** no
  mesmo dia DEVEM ser dispostos **lado a lado em colunas** (largura = `1/N` da coluna do dia,
  N = nº de eventos simultâneos no agrupamento de sobreposição), sem ficarem um por cima do
  outro. Eventos que apenas se **tocam na borda** (um termina exatamente quando o outro começa)
  NÃO são sobreposição (não dividem coluna).
- **CA-13** (all-day) — eventos `all_day:true` DEVEM aparecer numa **faixa fixa no topo** da
  grade (acima das horas, padrão GCal), na(s) coluna(s) do(s) dia(s) que cobrem; NUNCA na grade
  de horas. Um evento all-day de vários dias ocupa as colunas correspondentes.
- **CA-13.1** (all-day end EXCLUSIVO) — o `end` de um all-day é **exclusivo** (convenção do
  Google preservada pelo `internal/calendar/client.go`: um feriado de 1 dia em 15/06 chega como
  `start=2026-06-15T00:00`, `end=2026-06-16T00:00`). `allDayByDay` DEVE cobrir o intervalo
  **`[startDay, endDay)`** — o último dia coberto é o **dia anterior** a `end` quando `end` cai
  em `00:00`. Logo: all-day `D → D+1` ocupa **1** coluna (só `D`); all-day `D → D+3` ocupa **3**
  colunas (`D, D+1, D+2`), nunca N+1. (Sem isto, off-by-one pinta uma coluna a mais.)
- **CA-14** (fuso) — todo o posicionamento DEVE ser no **fuso local do navegador**
  (`browserTZ()` / `new Date(iso)`), consistente com a fase 06. Eventos vêm como ISO com offset.

### Grade & navegação (UI)

- **CA-15** (faixa = viewport, não corte) — a faixa de horas configurada (`hour_start..hour_end`)
  DEFINE o **viewport visível sem rolar** e o **alvo do auto-scroll** — NÃO recorta a grade:
  eventos fora da faixa continuam alcançáveis rolando (a grade tem 24h). Default 6h–23h.
- **CA-16** (linha do "agora") — nas views Dia/Semana, QUANDO o dia exibido inclui **hoje**,
  DEVE haver uma **linha do "agora"** na posição do horário local atual, **só na coluna de
  hoje**; ela **não aparece** em semanas/dias sem hoje. A posição atualiza ao longo do tempo
  (≤1min de defasagem).
- **CA-17** (auto-scroll) — ao abrir/trocar pra uma view Dia/Semana que contém hoje, a grade
  DEVE **rolar automaticamente** pra deixar o horário atual visível; sem hoje, rola pro
  `hour_start`.
- **CA-18** (grade do mês) — a view **Mês** DEVE renderizar uma grade de **semanas completas
  segunda→domingo** cobrindo o mês (semanas parciais incluem dias do mês anterior/seguinte,
  **visualmente atenuados**); cada célula mostra o **número do dia** e **chips de eventos**
  (com truncamento "+N" quando não cabem). O dia de **hoje** é destacado.
- **CA-19** (navegação por período) — a página DEVE navegar **anterior/próximo** e **"hoje"**,
  com o passo conforme a view (Dia: ±1 dia; Semana: ±1 semana; Mês: ±1 mês), e um **título do
  período** legível. Trocar de view preserva a data de referência (o dia/semana/mês "em foco").
- **CA-19.1** (±1 mês com clamp) — `addMonths(refDate, ±1)` DEVE **fixar o dia em
  `min(dia, últimoDiaDoMêsAlvo)`** (ou ancorar a navegação mensal no dia 1 do mês). A navegação
  ingênua `new Date(y, m±1, d)` é **proibida**: com `refDate=2026-01-31`, próximo mês DEVE dar
  **fevereiro/2026** (não março — fevereiro não tem dia 31), e o round-trip próximo→anterior
  DEVE preservar o mês. (O `refDate` sobrevive à troca de view, então um dia "sujo" vindo da
  view Dia não pode contaminar a navegação mensal.)
- **CA-19.2** (fonte do título) — o título do período DERIVA do **`refDate`**, NÃO da janela de
  fetch: Mês → o mês do `refDate` (ex.: "Julho 2026"), mesmo quando a grade de 42 dias começa
  em 29/06 e termina em 09/08; Dia → a data do `refDate`; Semana → seg→dom da semana do
  `refDate`. (Sem isto, o título do mês leria "29/06 – 09/08" atravessando 3 meses.)
- **CA-20** (semana começa na segunda) — Semana e Mês começam na **segunda** (consistente com
  `/reports/week` e a `weekRange` atual). Não é configurável nesta fase.

### Evento: detalhe (popover)

- **CA-21** — clicar/acionar um evento (com horário ou all-day) DEVE abrir um **popover** com:
  título (ou "(sem título)"), faixa de horário (ou "dia inteiro") e um link **"Abrir no
  Google"** apontando pro `html_link` do evento (`target=_blank`, `rel=noopener`). Acionar
  fora/Esc fecha. Evento sem `html_link` NÃO mostra o link (sem `href` quebrado).
- **CA-22** (cor) — os blocos de evento usam a **cor de accent** do app (token `--accent` /
  classes existentes); NÃO há cor por evento vinda do Google. (Decisão de design; a cor é só
  identidade visual, não dado novo.)

### Personalização na UI

- **CA-23** (view inicial) — ao abrir `/calendario`, a view inicial DEVE ser a `default_view`
  das preferências (CA-01). Trocar de view na barra **não** altera a preferência salva (a troca
  é de sessão); a preferência muda só pelos Ajustes (CA-25).
- **CA-24** (faixa/densidade aplicadas) — a grade DEVE aplicar `hour_start/hour_end` (CA-15/17)
  e a `density` (`comfortable` × `compact` → altura de linha / espaçamento) das preferências.
- **CA-24.1** (densidade escopada) — a `density` do calendário DEVE usar um token **próprio e
  escopado** ao container da grade (ex.: `--cal-row-h` aplicado só na TimeGrid via
  `style`/`className` local), **NUNCA** o token global `--row-h` nem o atributo
  `data-density="compact"` do documento — esses regridiriam a altura de linha de `/hoje`
  (`TodayList`) e `/workspace`, fora de escopo.
- **CA-25** (Ajustes) — a página **Ajustes** DEVE ganhar um grupo **"Calendário"** pra editar
  `default_view`, `hour_start`, `hour_end` e `density`, salvando via `PUT` (CA-02); formulário
  inválido (ex.: `hour_end ≤ hour_start`) NÃO chama a API e sinaliza o erro.

### Estados preservados (herdados da fase 06 — não regredir)

- **CA-26** — `status.connected=false` → CTA **"Conectar Google Calendar"** (sem buscar
  eventos). Erro no `/status` → estado de erro próprio (NUNCA cai em "desconectado"). Erro no
  `/events` → card de **"Reconectar"**. (Os três estados do `Calendar.tsx` da 06 seguem
  válidos em todas as views.)

### E2E

- **CA-27** — Logado e conectado (stub da 06), o usuário vê a **grade da semana** com os
  eventos da fixture posicionados; **troca pra Dia e pra Mês** e vê os mesmos eventos no novo
  layout; **clica num evento** e vê o popover com "Abrir no Google"; **muda a view padrão nos
  Ajustes**, recarrega `/calendario` e ela **abre na nova view padrão** (persistência backend).

## 4. Requisitos não-funcionais

- **RNF-01** (leveza) — **sem dependência pesada nova** (sem FullCalendar/biblioteca de
  calendário): a grade é construída com CSS grid/fl
  + funções puras de layout. PWA intacto.
- **RNF-02** (tokens) — **`tokens.css` é a única fonte de cor** (gate `check-cores`): a grade,
  a linha do "agora" e os blocos usam tokens/classes existentes (`--accent`, `--surface-*`,
  `--border`, `--text-*`). Cor nova só como token, se necessário. Fontes Geist; accent
  personalizável respeitado.
- **RNF-03** (migração) — a fase 11 **é dona da migração `0012_calendar_settings`** (próximo
  número livre; INDEX atualizado). Nenhuma outra tabela nova; nenhum cache de eventos
  (segue lendo ao vivo da 06).
- **RNF-04** (perf de layout) — o cálculo de posicionamento (top/height/colunas) é O(n log n)
  por dia e roda no cliente sem travar; uma semana típica (dezenas de eventos) renderiza
  instantânea. Mês busca **um** request só (CA-07).
- **RNF-05** — a fase 06 (rotas `/calendar/*`, `/reports/planned`, ângulo 4 dos relatórios)
  permanece **intocada** no backend; esta fase só ADICIONA `/settings/calendar`.

## 5. Pré-condições externas

- Nenhuma nova. A conexão real do Google Calendar (P-02) já foi feita pelo Diogo (sessão 13).
  Build e testes usam o Google stubbado (TDD.md §3); a persistência de preferências não depende
  de nada externo.

## 6. Mapa pro problema central (REQUISITOS)

- §4.3 / DESIGN §3.1 (cockpit: "alterna Calendário — blocos — e Lista") — entrega a metade
  **Calendário em blocos** (grade de horários). Drag-and-drop e timesheet-split seguem fora.
- DESIGN §4 (personalização: densidade, "o app é dele") — estende a personalização ao
  calendário, persistida no backend.

## 7. Fora de escopo (fronteiras explícitas)

- **Escrever no Google** (criar/editar/mover/excluir evento pela grade) — segue fora (fase 06
  já recortou o write-back).
- **Drag-and-drop** de eventos/entries e **criar entry arrastando** na grade — fase futura.
- **Cor por evento** vinda do Google (`colorId`) — descartado por decisão de design (CA-22); o
  backend de calendar não muda.
- **Multi-agenda** / agendas de colaboradores / seletor de agendas — segue fora (06 §7).
- **Início da semana configurável** (domingo) — fixo na segunda nesta fase (CA-20).
- **Sobrepor time entries cronometradas na grade** (planejado×real visual dentro da grade) — o
  ângulo 4 vive nos Relatórios (06); trazer pra grade é fase futura.
