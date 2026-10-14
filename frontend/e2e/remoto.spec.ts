import { test, expect, type Page } from '@playwright/test';
import { loginViaStub } from './helpers';
import { acquireTimerLock, releaseTimerLock } from './timerLock';

// remoto.spec — fase 23 T-10 (CA-20): o fluxo do controle remoto em viewport
// MOBILE (375×667). Logado (stub): /remoto renderiza em UMA coluna → seleciona
// um projeto (criado via API) na fila/seletor → Iniciar → timer rodando ao vivo
// (dígitos avançam, descrição/chip certos) → Parar → ocioso.
//
// SERIAL + ensureIdle no início: o timer é GLOBAL por usuário (o googlestub só
// autentica diogo.fixture@example.com e o db-test é reusado entre runs) — em
// paralelo um start mataria o timer do outro (CA-05). Mesmo padrão do
// cronometro.spec / dashboard.spec.
//
// Exceção ao red→green estrutural (precedente do cronometro.spec/T-19 da 03):
// o app exercitado JÁ existe (T-02..T-09 são pré-requisito) — o "red honesto" é
// este spec rodado ANTES de qualquer ajuste de app.
//
// CA-21 (presence de um colega) — LIMITAÇÃO REGISTRADA: a presence ao vivo
// depende do broadcast por workspace VIVO (o socket entrega o type:"presence"
// só quando o backend dispara o delta do colega), que a fase 21 prova nos
// próprios testes de integração Go. Headless aqui o snapshot
// GET /workspaces/{id}/presence até semearia um colega rodando (seedColleagueRunning),
// MAS a faixa do remoto filtra o próprio usuário e o stub só tem 1 user logado
// (o "colega" não tem sessão pra alimentar o broadcast). A presence do remoto já
// está COBERTA por teste de unidade (RemotePresence.spec.tsx — CA-08/08.1/08.2,
// jsdom, store mockado), como o Web Push da fase 16 / o sync passivo da 04. O
// fluxo do CA-20 abaixo é o E2E obrigatório.
test.describe.configure({ mode: 'serial' });
test.use({ viewport: { width: 375, height: 667 } });

// mutex do timer global (mesmo de cronometro.spec): os 2 specs que mutam o timer
// não rodam concorrentes — senão o ensureIdle/Parar daqui apaga o timer que o
// cronometro.spec testa no reload (CA-37). Adquire enquanto roda, libera no fim.
test.beforeAll(acquireTimerLock);
test.afterAll(releaseTimerLock);

// deixa o usuário ocioso (stop encerra timer/pausa; 409 ocioso é ok). O db-test
// é reusado — um timer vazado de run anterior quebraria o ramo ocioso.
async function ensureIdle(page: Page) {
  await page.request.post('/api/v1/timer/stop');
}

// "h:mm:ss" ou "mm:ss" → segundos
function parseClock(txt: string): number {
  const p = txt.trim().split(':').map(Number);
  return p.length === 3 ? p[0] * 3600 + p[1] * 60 + p[2] : p[0] * 60 + p[1];
}

test('CA-20: fluxo remoto em 375px — selecionar projeto → Iniciar → rodando ao vivo → Parar', async ({
  page,
}) => {
  await loginViaStub(page);
  await ensureIdle(page);

  const sufixo = Date.now();
  const projeto = `Projeto Remoto ${sufixo}`;
  const descricao = `Sessão remota ${sufixo}`;

  // cria o projeto via API (padrão dos specs serial — sem depender da UI de
  // projetos no fluxo do remoto)
  const me = (await (await page.request.get('/api/v1/me')).json()) as {
    workspaces: { id: string }[];
  };
  const wsId = me.workspaces[0].id;
  const novo = await page.request.post(`/api/v1/workspaces/${wsId}/projects`, {
    data: { name: projeto, color: '#4f9dde', visibility: 'shared' },
  });
  expect(novo.status()).toBe(201);

  // abre o /remoto — coluna única com o controle visível
  await page.goto('/remoto');
  const remote = page.getByTestId('remote');
  await expect(remote).toBeVisible();
  await expect(remote.getByRole('button', { name: 'Iniciar' })).toBeVisible();

  // descrição + seleciona o projeto no seletor (a fila de recentes pode estar
  // vazia num banco fresco; o seletor é o caminho garantido)
  await remote.getByTestId('remote-description').fill(descricao);
  await remote.getByLabel('Projeto').selectOption({ label: projeto });

  // Iniciar → timer rodando ao vivo
  await remote.getByRole('button', { name: 'Iniciar' }).click();

  // descrição read-only com o texto certo + chip do projeto + dígitos h:mm:ss
  await expect(remote.getByTestId('remote-description')).toHaveValue(descricao);
  await expect(remote.locator('.tt-chip')).toContainText(projeto);
  await expect(remote.getByTestId('remote-digits')).toHaveText(/^\d+:\d{2}:\d{2}$/);

  // dígitos AVANÇAM (timer vivo derivado de server_time+offset)
  const t0 = parseClock((await remote.getByTestId('remote-digits').textContent()) ?? '');
  await expect
    .poll(async () => parseClock((await remote.getByTestId('remote-digits').textContent()) ?? ''), {
      timeout: 4000,
    })
    .toBeGreaterThan(t0);

  // Parar (>1s rodando — o check do banco exige stopped_at > started_at)
  await remote.getByRole('button', { name: 'Parar' }).click();
  await expect(remote.getByRole('button', { name: 'Iniciar' })).toBeVisible();
});

test('CA-02: o item "Remoto" aparece na nav mobile (menu Mais) e navega', async ({ page }) => {
  await loginViaStub(page);
  await ensureIdle(page);

  await page.goto('/');
  // viewport mobile → a bottom bar mostra "Mais"; o /remoto cai no menu
  await page.getByRole('button', { name: /mais/i }).click();
  await page.getByRole('link', { name: /remoto/i }).click();
  await expect(page).toHaveURL(/\/remoto$/);
  await expect(page.getByTestId('remote')).toBeVisible();
});
