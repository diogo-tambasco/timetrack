import { test, expect, type Browser, type Page } from '@playwright/test';
import { loginViaStub, seedForgottenTimer } from './helpers';
import { acquireTimerLock, releaseTimerLock } from './timerLock';

// T-19 — E2E da fase 03 contra o app real (backend Go + Postgres de teste):
// CA-36 (editar horários pelo modal recalcula a duração na lista), CA-37
// (reload restaura a barra com elapsed certo), CA-38 (pausa configurada nos
// Ajustes vale na próxima pausa) + o fluxo nº1 do TDD.md §4 (cria projeto →
// timer → para → entrada com projeto certo).
//
// Exceção ao red→green (documentada, como o timer_perf_test.go do T-20): o
// T-19 tem como entrada T-14–T-18, ou seja, o app exercitado JÁ existia —
// red estrutural impossível. O "red honesto" aplicado foi o do kickoff:
// spec rodado ANTES de qualquer ajuste de app (3/4 verdes de primeira; a
// única falha era locator do próprio teste).
//
// SERIAL de propósito: o timer é GLOBAL por usuário (CA-03) e o googlestub
// só tem 1 usuário — em paralelo, o start de um teste mataria o timer do
// outro pela troca atômica do CA-05.
test.describe.configure({ mode: 'serial' });

// mutex do timer global: este arquivo e o remoto.spec (os 2 que mutam o timer)
// não podem rodar concorrentes sob fullyParallel — senão um stop/start de um
// apaga o timer do outro (flake do CA-37). Adquire enquanto roda, libera no fim.
test.beforeAll(acquireTimerLock);
test.afterAll(releaseTimerLock);

// Deixa o usuário ocioso: stop encerra timer rodando E pausa ativa
// (CA-06.1); ocioso responde 409 e tudo bem. O banco é reusado entre runs —
// um timer vazado de run anterior quebraria a barra ociosa.
async function ensureIdle(page: Page) {
  await page.request.post('/api/v1/timer/stop');
}

// "h:mm:ss" ou "mm:ss" → segundos
function parseClock(txt: string): number {
  const p = txt.trim().split(':').map(Number);
  return p.length === 3 ? p[0] * 3600 + p[1] * 60 + p[2] : p[0] * 60 + p[1];
}

// data de HOJE no fuso local (inputs datetime-local do EntryModal)
function hojeLocal(): string {
  const d = new Date();
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

test('fluxo nº1: cria projeto → timer → para → entrada com projeto certo', async ({ page }) => {
  await loginViaStub(page);
  await ensureIdle(page);

  const sufixo = Date.now();
  const projeto = `Projeto Cronômetro ${sufixo}`;
  const descricao = `Revisão de laudas ${sufixo}`;

  // 1) cria o projeto (cliente é opcional)
  await page.goto('/projetos');
  await page.getByRole('button', { name: /novo projeto/i }).click();
  const dialog = page.getByRole('dialog', { name: /novo projeto/i });
  await dialog.getByLabel(/nome do projeto/i).fill(projeto);
  await dialog.getByRole('button', { name: /criar projeto/i }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);

  // 2) inicia o timer na barra global, já com o projeto
  await page.goto('/hoje');
  const bar = page.getByTestId('timerbar');
  await bar.getByPlaceholder(/no que você está trabalhando/i).fill(descricao);
  await bar.getByLabel('Projeto').selectOption({ label: projeto });
  await bar.getByRole('button', { name: 'Iniciar' }).click();

  // rodando: chip do projeto + stop pulsante (CA-31 ao vivo)
  await expect(bar.locator('.tt-chip')).toContainText(projeto);
  await expect(bar.locator('.tt-pulse')).toBeVisible();

  // 3) para (>1s rodando — o check do banco exige stopped_at > started_at)
  await page.waitForTimeout(1500);
  await bar.getByRole('button', { name: 'Parar' }).click();
  await expect(bar.getByRole('button', { name: 'Iniciar' })).toBeVisible();

  // 4) a entrada está na lista "Hoje" com o projeto certo
  const linha = page.locator('[data-testid^="entry-"]', { hasText: descricao });
  await expect(linha).toBeVisible();
  await expect(linha).toContainText(projeto);
});

test('CA-36: editar horários pelo modal atualiza a duração na lista', async ({ page }) => {
  await loginViaStub(page);
  await ensureIdle(page);

  const descricao = `Entrada manual ${Date.now()}`;
  const hoje = hojeLocal();

  // cria a entrada manual de 45min pelo modal da página /hoje
  await page.goto('/hoje');
  await page.getByRole('button', { name: /nova entrada/i }).click();
  const modal = page.getByRole('dialog', { name: /nova entrada/i });
  await modal.getByLabel(/descrição/i).fill(descricao);
  await modal.getByLabel('Início').fill(`${hoje}T08:00`);
  await modal.getByLabel('Fim').fill(`${hoje}T08:45`);
  await modal.getByRole('button', { name: /salvar/i }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);

  const linha = page.locator('[data-testid^="entry-"]', { hasText: descricao });
  await expect(linha).toContainText('0:45:00');

  // edita o fim 08:45 → 10:30 e a lista mostra a duração RECALCULADA
  // pelo servidor (2:30:00), sem reload
  await linha.getByRole('button', { name: 'Editar' }).click();
  const edit = page.getByRole('dialog', { name: /editar entrada/i });
  await edit.getByLabel('Fim').fill(`${hoje}T10:30`);
  await edit.getByRole('button', { name: /salvar/i }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(linha).toContainText('2:30:00');
});

test('CA-37: reload com timer rodando restaura a barra com elapsed certo (≤2s)', async ({
  page,
}) => {
  await loginViaStub(page);
  await ensureIdle(page);

  const descricao = `Timer sobrevive a reload ${Date.now()}`;
  await page.goto('/hoje');
  const bar = page.getByTestId('timerbar');
  await bar.getByPlaceholder(/no que você está trabalhando/i).fill(descricao);

  // âncora local na RESPOSTA do start: o started_at do servidor já existe
  // aqui, então a latência click→servidor (que cresce sob carga) fica FORA
  // do erro medido — o teste mede só o que o CA-37 pede (exibição ≤2s)
  const startResp = page.waitForResponse(
    (r) => r.url().includes('/timer/start') && r.request().method() === 'POST',
  );
  await bar.getByRole('button', { name: 'Iniciar' }).click();
  await startResp;
  const t0 = Date.now();
  await expect(bar.locator('.tt-pulse')).toBeVisible();

  await page.waitForTimeout(3000);
  await page.reload();

  // a barra restaura do GET /timer: mesma descrição, sem zerar
  await expect(bar.locator('input')).toHaveValue(descricao);
  const exibido = parseClock((await bar.locator('.mono').textContent()) ?? '');
  const esperado = (Date.now() - t0) / 1000;
  expect(Math.abs(exibido - esperado)).toBeLessThanOrEqual(2);

  await bar.getByRole('button', { name: 'Parar' }).click();
  await expect(bar.getByRole('button', { name: 'Iniciar' })).toBeVisible();
});

test('CA-38: pausa de 15 min nos Ajustes vale na próxima pausa', async ({ page }) => {
  await loginViaStub(page);
  await ensureIdle(page);

  // o banco é reusado entre runs: salvar 5 ANTES prova que o 15 que vem
  // depois é efeito desta edição, não sobra de run anterior — e o CORPO do
  // PUT é assertado (não só o status), senão um preset morto enviando o
  // valor semeado (que num run repetido é justamente 15) passaria batido
  await page.goto('/ajustes');
  // fase 11: o /ajustes ganhou um 2º grupo (Calendário) com botão "Salvar" —
  // escopa o clique ao card do Pomodoro (desvio sancionado: a página mudou).
  const pomoCard = page.locator('section.tt-card', { hasText: 'Pomodoro' });
  const salvarPausa = async (preset: string, minutos: number) => {
    await page.getByRole('button', { name: preset, exact: true }).click();
    const put = page.waitForResponse(
      (r) => r.url().includes('/settings/pomodoro') && r.request().method() === 'PUT',
    );
    await pomoCard.getByRole('button', { name: /^salvar$/i }).click();
    const resp = await put;
    expect(resp.status()).toBe(200);
    expect(resp.request().postDataJSON().break_minutes).toBe(minutos);
  };
  await salvarPausa('5 min', 5);
  await salvarPausa('15 min', 15);

  // inicia trabalho e pausa: a pausa abre com ~15:00 (não os 5 antigos)
  const bar = page.getByTestId('timerbar');
  await bar.getByPlaceholder(/no que você está trabalhando/i).fill(`Pausa de 15 ${Date.now()}`);
  await bar.getByRole('button', { name: 'Iniciar' }).click();
  await expect(bar.locator('.tt-pulse')).toBeVisible();
  await bar.getByRole('button', { name: 'Pausar' }).click();

  const overlay = page.getByTestId('break-overlay');
  await expect(overlay).toBeVisible();
  await expect(overlay).toContainText('Pausa Pomodoro');
  const restante = parseClock((await overlay.locator('.mono').textContent()) ?? '');
  expect(restante).toBeGreaterThan(880); // 14:40+ — nunca os 300s do preset 5
  expect(restante).toBeLessThanOrEqual(900); // 15:00 cravado no servidor

  // limpa: stop durante a pausa deixa ocioso (CA-06.1)
  await overlay.getByRole('button', { name: 'Parar' }).click();
  await expect(bar.getByRole('button', { name: 'Iniciar' })).toBeVisible();
});

// ───────────────────────── fase 04 — sync em tempo real ─────────────────────────
// T-10: 2 contextos = 2 "devices" do MESMO usuário do stub (cenário
// desejado — plan §6), no arquivo serial existente (timer global). Exceção
// ao red estrutural documentada (precedente do T-19): T-05–T-08 são
// pré-requisito, então o app já está pronto; red honesto = este spec rodado
// ANTES de qualquer ajuste que ele exija. Asserções do device B sempre com
// auto-retry ≤3s (CA-21) — nunca waitForTimeout como única espera.

// abre o "segundo device": contexto novo, mesmo usuário do googlestub.
async function abrirDeviceB(browser: Browser) {
  const context = await browser.newContext();
  const page = await context.newPage();
  await loginViaStub(page);
  return { context, page };
}

test('sync CA-21/CA-13: start no A reflete no B em ≤3s; stop volta a ocioso', async ({
  page,
  browser,
}) => {
  await loginViaStub(page);
  await ensureIdle(page);

  const sufixo = Date.now();
  const projeto = `Projeto Sync ${sufixo}`;
  const descricao = `Sync start ${sufixo}`;

  // projeto criado ANTES do device B abrir: o chip do B resolve o nome pela
  // query de projetos carregada no boot do B (decisão 5 do bloco 2 da 03)
  await page.goto('/projetos');
  await page.getByRole('button', { name: /novo projeto/i }).click();
  const dialog = page.getByRole('dialog', { name: /novo projeto/i });
  await dialog.getByLabel(/nome do projeto/i).fill(projeto);
  await dialog.getByRole('button', { name: /criar projeto/i }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);

  const devB = await abrirDeviceB(browser);
  const barB = devB.page.getByTestId('timerbar');
  await expect(barB.getByRole('button', { name: 'Iniciar' })).toBeVisible();

  // start no A — a partir daqui o B NÃO navega nem recarrega
  await page.goto('/hoje');
  const barA = page.getByTestId('timerbar');
  await barA.getByPlaceholder(/no que você está trabalhando/i).fill(descricao);
  await barA.getByLabel('Projeto').selectOption({ label: projeto });
  await barA.getByRole('button', { name: 'Iniciar' }).click();

  // CA-21: barra do B rodando com a MESMA descrição + chip do projeto ≤3s
  await expect(barB.locator('input')).toHaveValue(descricao, { timeout: 3000 });
  await expect(barB.locator('.tt-chip')).toContainText(projeto, { timeout: 3000 });
  await expect(barB.locator('.tt-pulse')).toBeVisible({ timeout: 3000 });

  // CA-13: stop no A → barra do B volta a ociosa (continue herda o caminho
  // do start — nota na matriz da spec)
  await page.waitForTimeout(1500); // o check do banco exige stopped_at > started_at
  await barA.getByRole('button', { name: 'Parar' }).click();
  await expect(barB.getByRole('button', { name: 'Iniciar' })).toBeVisible({ timeout: 3000 });

  await devB.context.close();
});

test('sync CA-20/CA-12: pause no A abre o BreakOverlay no B sem nenhuma mutação própria', async ({
  page,
  browser,
}) => {
  await loginViaStub(page);
  await ensureIdle(page);

  const devB = await abrirDeviceB(browser);
  const barB = devB.page.getByTestId('timerbar');
  await expect(barB.getByRole('button', { name: 'Iniciar' })).toBeVisible();

  // o device passivo NUNCA emite mutação de timer (asserção absorvida da
  // ex-T-09): qualquer POST /timer* do B é falha
  const postsDoB: string[] = [];
  devB.page.on('request', (req) => {
    if (req.method() === 'POST' && req.url().includes('/timer')) postsDoB.push(req.url());
  });

  // A trabalha e pausa
  const barA = page.getByTestId('timerbar');
  await barA.getByPlaceholder(/no que você está trabalhando/i).fill(`Pausa sync ${Date.now()}`);
  await barA.getByRole('button', { name: 'Iniciar' }).click();
  await expect(barB.locator('.tt-pulse')).toBeVisible({ timeout: 3000 }); // B sincronizou o start
  await page.waitForTimeout(1500); // stopped_at > started_at
  await barA.getByRole('button', { name: 'Pausar' }).click();

  // CA-20/CA-12: B troca pro modo descanso com countdown — sem reload, sem
  // ação local no B
  const overlayB = devB.page.getByTestId('break-overlay');
  await expect(overlayB).toBeVisible({ timeout: 3000 });
  await expect(overlayB).toContainText('Pausa Pomodoro');
  await expect
    .poll(async () => parseClock((await overlayB.locator('.mono').textContent()) ?? ''), {
      timeout: 3000,
    })
    .toBeGreaterThan(0);

  // CA-13 (resume): retomar no A → B volta a rodando
  await page.getByTestId('break-overlay').getByRole('button', { name: 'Retomar' }).click();
  await expect(barB.locator('.tt-pulse')).toBeVisible({ timeout: 3000 });
  await expect(devB.page.getByTestId('break-overlay')).toHaveCount(0);

  expect(postsDoB).toEqual([]);

  // limpa pro próximo teste
  await page.getByTestId('timerbar').getByRole('button', { name: 'Parar' }).click();
  await expect(page.getByTestId('timerbar').getByRole('button', { name: 'Iniciar' })).toBeVisible();
  await devB.context.close();
});

test('sync CA-17: countdown zerando com 2 devices abertos = exatamente 1 entry fechada, sem erro', async ({
  page,
  browser,
}) => {
  await loginViaStub(page);
  await ensureIdle(page);

  const devB = await abrirDeviceB(browser);
  const barA = page.getByTestId('timerbar');
  const barB = devB.page.getByTestId('timerbar');
  await expect(barB.getByRole('button', { name: 'Iniciar' })).toBeVisible();

  // nenhum dialog/erro pode aparecer em NENHUM device (o 409 do perdedor é
  // engolido pelo catch → load() do checkAutoTransition — plan §6)
  const dialogs: string[] = [];
  page.on('dialog', (d) => {
    dialogs.push(d.message());
    void d.dismiss();
  });
  devB.page.on('dialog', (d) => {
    dialogs.push(d.message());
    void d.dismiss();
  });

  const me = (await (await page.request.get('/api/v1/me')).json()) as {
    workspaces: { id: string }[];
  };
  const wsId = me.workspaces[0].id;
  const descricao = `Corrida countdown ${Date.now()}`;

  // countdown de 2s VIA API (a fase 03 não tem UI de start de countdown; a
  // barra só inicia countup) — os 2 devices recebem o estado pelo WS e a
  // transição automática dispara "ao mesmo tempo" nos dois
  const resp = await page.request.post('/api/v1/timer/start', {
    data: { workspace_id: wsId, description: descricao, mode: 'countdown', target_seconds: 2 },
  });
  expect(resp.status()).toBe(201);

  // ao zerar: exatamente 1 entry FECHADA com essa descrição no banco
  const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
  const listar = async () => {
    const r = await page.request.get(
      `/api/v1/workspaces/${wsId}/entries?tz=${encodeURIComponent(tz)}`,
    );
    const body = (await r.json()) as {
      entries: { description: string; stopped_at: string | null }[];
    };
    return body.entries.filter((e) => e.description === descricao);
  };
  await expect
    .poll(async () => (await listar()).filter((e) => e.stopped_at !== null).length, {
      timeout: 10_000,
    })
    .toBe(1);
  expect(await listar()).toHaveLength(1); // nem duplicada, nem outra rodando

  // ambos os devices ociosos, nenhum erro visível — nem dialog (canal que
  // o app não usa hoje; custo zero manter) nem role=alert (o canal REAL de
  // erro do app — verificação adversarial do build, 11/06)
  await expect(barA.getByRole('button', { name: 'Iniciar' })).toBeVisible({ timeout: 3000 });
  await expect(barB.getByRole('button', { name: 'Iniciar' })).toBeVisible({ timeout: 3000 });
  expect(dialogs).toEqual([]);
  await expect(page.getByRole('alert')).toHaveCount(0);
  await expect(devB.page.getByRole('alert')).toHaveCount(0);

  await devB.context.close();
});

// ───────────────────────── fase 05/15 — Modo Foco ─────────────────────────
// fase 05 T-06 (CA-18) ATUALIZADA na fase 15 T-12 (CA-21): o fluxo nº3 abre
// agora pelo FocoPicker (o ocioso virou a tela de entrada — desvio sancionado
// §7), não mais pela TimerBar. No arquivo SERIAL (o teste muta o timer). TODA
// query do fluxo é escopada em getByTestId('focus') — a TimerBar/BreakOverlay
// continuam montadas POR BAIXO do overlay com botões gêmeos (strict mode).

test('fase 15 CA-21: fluxo nº3 — Modo Foco via FocoPicker (entrada → foco → pausa → retomar → parar → picker → sair)', async ({
  page,
}) => {
  await loginViaStub(page);
  await ensureIdle(page);

  // Pomodoro fixado VIA API (determinismo — corpo COMPLETO, CA-02): pausa
  // curta 5 / longa 15 / cycles 2 / AUTO-START OFF (fluxo determinístico —
  // a regra longa×curta é provada no backend, CA-08.1).
  const put = await page.request.put('/api/v1/settings/pomodoro', {
    data: {
      focus_minutes: 25,
      break_minutes: 5,
      cycles_before_long: 2,
      long_break_minutes: 15,
      auto_start_break: false,
      auto_start_focus: false,
      sound_enabled: false,
    },
  });
  expect(put.status()).toBe(200);

  const sufixo = Date.now();
  const projeto = `Projeto Foco ${sufixo}`;

  // cria o projeto (vira cartão recente no FocoPicker)
  await page.goto('/projetos');
  await page.getByRole('button', { name: /novo projeto/i }).click();
  const dialog = page.getByRole('dialog', { name: /novo projeto/i });
  await dialog.getByLabel(/nome do projeto/i).fill(projeto);
  await dialog.getByRole('button', { name: /criar projeto/i }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);

  // garante que "Projeto Foco" seja o cartão recente nº1 do FocoPicker mesmo num
  // db-test ACUMULADO: recentProjects rankeia por entries started_at DESC (mais
  // recente primeiro) e corta no top-5 (FocoPicker.tsx + entries/repo.go ORDER BY
  // started_at DESC). Uma entry FECHADA começada AGORA-2s é a mais recente de hoje
  // → rank 0 → SEMPRE no top-5, não importa quantos projetos já se acumularam. (Com
  // -60s ela caía fora do top-5 quando entries mais recentes de outros testes se
  // empilhavam no db reusado → flake mesmo SERIAL, achado no gate da fase 24.)
  const me = (await (await page.request.get('/api/v1/me')).json()) as {
    workspaces: { id: string }[];
  };
  const wsId = me.workspaces[0].id;
  const projs = (await (await page.request.get(`/api/v1/workspaces/${wsId}/projects`)).json()) as {
    projects: { id: string; name: string }[];
  };
  const projId = projs.projects.find((p) => p.name === projeto)!.id;
  const agora = Date.now();
  const seed = await page.request.post(`/api/v1/workspaces/${wsId}/entries`, {
    data: {
      description: `prévia ${sufixo}`,
      project_id: projId,
      started_at: new Date(agora - 2_000).toISOString(),
      stopped_at: new Date(agora - 1_000).toISOString(),
    },
  });
  expect(seed.status()).toBe(201);

  // abre o /foco OCIOSO → FocoPicker (tela de entrada)
  await page.goto('/foco');
  const foco = page.getByTestId('focus');
  await expect(foco).toBeVisible();
  await expect(foco.getByTestId('focus-picker')).toBeVisible();
  await expect(foco.getByText('No que você tá trabalhando?')).toBeVisible();

  // digita a descrição + toca o cartão do projeto recente → Começar
  await foco.getByTestId('focus-picker-description').fill(`Sessão de foco ${sufixo}`);
  await foco.getByRole('button', { name: new RegExp(projeto, 'i') }).first().click();
  await foco.getByTestId('focus-picker-start').click();

  // tela de trabalho: dígitos gigantes + contexto (projeto) + rodapé hoje:
  await expect(foco.getByTestId('focus-digits')).toHaveText(/\d+:\d{2}:\d{2}/);
  await expect(foco.getByTestId('focus-context')).toHaveText(projeto);
  await expect(foco.getByTestId('focus-day-total')).toContainText('hoje:');

  // Pausar DENTRO do foco → FocoBreak (modo descanso) com a pausa curta (5min)
  await page.waitForTimeout(1500); // stopped_at > started_at (check do banco)
  await foco.getByRole('button', { name: 'Pausar' }).click();
  await expect(foco.getByTestId('focus-break')).toBeVisible();
  await expect(foco.getByText('Pausa Pomodoro')).toBeVisible();
  const restante = parseClock((await foco.getByTestId('focus-digits').textContent()) ?? '');
  expect(restante).toBeGreaterThan(280); // 4:40+ — pausa curta de 5min
  expect(restante).toBeLessThanOrEqual(300); // 5:00 cravado no servidor

  // retomar → dígitos de trabalho de volta (h:mm:ss)
  await foco.getByRole('button', { name: 'Retomar' }).click();
  await expect(foco.getByTestId('focus-digits')).toHaveText(/\d+:\d{2}:\d{2}/);

  // parar → FocoPicker de volta (ocioso = tela de entrada, fase 15)
  await page.waitForTimeout(1500); // entry nova do resume também precisa de >1s
  await foco.getByRole('button', { name: 'Parar' }).click();
  await expect(foco.getByTestId('focus-picker')).toBeVisible();

  // sair → home (o ✕ vai pra /hoje, que redireciona pra / — fase 13 CA-05),
  // overlay desmontado
  await foco.getByRole('button', { name: 'Sair' }).click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByTestId('focus')).toHaveCount(0);
});

// ───────────────────────── fase 13 — Dashboard (card do timer) ─────────────────────────
// CA-25 (partes que MUTAM o timer): a home `/` mostra o card do timer rodando
// (dígitos vivos + projeto) e continuar uma entrada reflete no card/TimerBar.
// Vive AQUI (arquivo serial) porque o timer é global por usuário (fullyParallel
// mataria a corrida com os outros specs). As partes paralelas (P3/recentes,
// atalho Foco, nav Hoje) ficam no dashboard.spec.ts.
test('fase 13 CA-25: home mostra o card do timer rodando + continuar reflete no card', async ({
  page,
}) => {
  await loginViaStub(page);
  await ensureIdle(page);

  const sufixo = Date.now();
  const projeto = `Projeto Card ${sufixo}`;
  const descricao = `Sessão card ${sufixo}`;

  // projeto (pro chip do card)
  await page.goto('/projetos');
  await page.getByRole('button', { name: /novo projeto/i }).click();
  const dialog = page.getByRole('dialog', { name: /novo projeto/i });
  await dialog.getByLabel(/nome do projeto/i).fill(projeto);
  await dialog.getByRole('button', { name: /criar projeto/i }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);

  // inicia o timer na barra global, na própria home
  await page.goto('/');
  const bar = page.getByTestId('timerbar');
  await bar.getByPlaceholder(/no que você está trabalhando/i).fill(descricao);
  await bar.getByLabel('Projeto').selectOption({ label: projeto });
  await bar.getByRole('button', { name: 'Iniciar' }).click();

  // o CARD do dashboard (não a barra) mostra rodando: rótulo + chip + dígitos vivos
  const card = page.getByTestId('timer-card');
  await expect(card.getByText(/rodando agora/i)).toBeVisible();
  await expect(card.locator('.tt-chip')).toContainText(projeto);
  await expect(card.locator('.mono')).toHaveText(/^\d+:\d{2}:\d{2}$/);

  // para pelo card → vira ocioso
  await page.waitForTimeout(1500); // stopped_at > started_at (check do banco)
  await card.getByRole('button', { name: 'Parar' }).click();
  await expect(card.getByText(/sem timer rodando/i)).toBeVisible();

  // continuar a entrada (linha de HOJE no TodayList reusado) reflete no card E na barra
  const linha = page.locator('[data-testid^="entry-"]', { hasText: descricao });
  await linha.getByRole('button', { name: 'Continuar' }).click();
  await expect(card.getByText(/rodando agora/i)).toBeVisible();
  await expect(bar.locator('.tt-pulse')).toBeVisible();

  // limpa
  await page.waitForTimeout(1500);
  await card.getByRole('button', { name: 'Parar' }).click();
  await expect(card.getByText(/sem timer rodando/i)).toBeVisible();
});

// fase 16 CA-19: timer esquecido >2h é cortado pelo worker de background —
// banner + ocioso + entry de 2h auto-cortada. FICA NESTE arquivo serial (não
// num spec próprio): ele dirige o timer GLOBAL do único usuário do stub; um
// arquivo paralelo clobbearia o seedForgottenTimer ↔ o timer vivo dos outros
// testes (e vice-versa). O worker do E2E tica a cada 1s (GUARD_TICK no
// playwright.config) e o limiar de 2h é satisfeito semeando started_at no
// passado (o clock do worker é o real).
test('fase 16 CA-19: timer esquecido >2h é cortado — banner + ocioso + entry de 2h auto-cortada', async ({
  page,
}) => {
  await loginViaStub(page);
  await ensureIdle(page);

  const descricao = `Timer esquecido ${Date.now()}`;
  await expect(page.getByTestId('timerbar')).toBeVisible();

  // semeia o timer rodando há 3h (>2h) + timer_states; o worker corta no marco
  seedForgottenTimer(descricao);

  // o notice chega pelo WS e o banner aparece em qualquer rota (vive no shell)
  await expect(page.getByTestId('notice-banner')).toBeVisible({ timeout: 15_000 });
  await expect(page.getByTestId('notice-banner')).toContainText(/pausado automaticamente após 2h/i);

  // GET /timer reporta ocioso (o corte limpou o timer_states — CA-04)
  await expect
    .poll(
      async () => {
        const r = await page.request.get('/api/v1/timer');
        return (await r.json()).state;
      },
      { timeout: 10_000 },
    )
    .toBeNull();

  // a entry foi fechada com 2h exatas e auto_cut (CA-03)
  const entries = await page.evaluate(async (desc) => {
    const me = await fetch('/api/v1/me', { credentials: 'include' }).then((r) => r.json());
    const wsId = me.workspaces[0].id;
    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    const now = new Date();
    const from = new Date(now.getTime() - 7 * 86400_000).toISOString().slice(0, 10);
    const to = new Date(now.getTime() + 86400_000).toISOString().slice(0, 10);
    const res = await fetch(
      `/api/v1/workspaces/${wsId}/entries?from=${from}&to=${to}&tz=${encodeURIComponent(tz)}`,
      { credentials: 'include' },
    ).then((r) => r.json());
    return res.entries.filter((e: { description: string }) => e.description === desc);
  }, descricao);

  expect(entries.length).toBe(1);
  expect(entries[0].duration_seconds).toBe(7200); // marco, não now (CA-03)
  expect(entries[0].stopped_at).not.toBeNull();
  const dStart = new Date(entries[0].started_at).getTime();
  const dStop = new Date(entries[0].stopped_at).getTime();
  expect(dStop - dStart).toBe(2 * 3600 * 1000);

  // ✕ dispensa o banner sem efeito no servidor
  await page.getByRole('button', { name: /dispensar aviso/i }).click();
  await expect(page.getByTestId('notice-banner')).toBeHidden();
  await ensureIdle(page);
});
