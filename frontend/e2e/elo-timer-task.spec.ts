import { test, expect, type Page } from '@playwright/test';
import { loginViaStub } from './helpers';
import { acquireTimerLock, releaseTimerLock } from './timerLock';

// elo-timer-task.spec — fase 30 T-10 (CA-13/16/18 + 30-H5): o ELO timer↔task.
// Logado (stub) num projeto shared: abrir uma task no popup (fase 24) → o controle
// de tempo (TaskTimeControl no lugar do bloco Time mock) mostra Start → clicar Start
// cronometra a task → a TimerBar global mostra o CHIP da task (CA-16) e o controle
// vira Parar (CA-11) → Parar → o tempo GRUDA na task (CA-13). Caso 30-H5: iniciar
// pela task, FECHAR o popup, parar pela TimerBar global → o board converge (o
// observador no ProjectDetail invalida mesmo com o popup fechado). Controle: timer
// SEM task (label livre) não mostra chip de task (CA-18).
//
// SERIAL + mutex do timer global (timerLock, fase 23) + ensureIdle: o timer é GLOBAL
// por usuário (o googlestub só autentica diogo.fixture@example.com, db-test reusado)
// → em paralelo um start mataria o timer de cronometro.spec/remoto.spec (CA-05).
//
// Granularidade: o tempo no controle/board é fmtDayTotal ("h:mm") — uma sessão E2E
// de ~1s NÃO move o texto (minuto). O "tempo grudou" (US-02/CA-13) é assertado pela
// API do /board (tracked_seconds > 0, determinístico); a invalidação-sem-reload do
// 30-H5 é provada por um GET /board disparado APÓS o Parar com o popup fechado.
//
// Red honesto: antes da costura (T-08.1) o task-start era mock (disabled) → clicá-lo
// não moveria a TimerBar; este spec falharia. Com o elo fiado (T-03..T-08.1), passa.
test.describe.configure({ mode: 'serial' });
test.beforeAll(acquireTimerLock);
test.afterAll(releaseTimerLock);

async function ensureIdle(page: Page) {
  await page.request.post('/api/v1/timer/stop'); // 409 ocioso é ok
}

async function setup(page: Page, sufixo: number) {
  const me = (await (await page.request.get('/api/v1/me')).json()) as { workspaces: { id: string }[] };
  const wsId = me.workspaces[0].id;
  const proj = await (
    await page.request.post(`/api/v1/workspaces/${wsId}/projects`, {
      data: { name: `Projeto Elo ${sufixo}`, color: '#4f9dde', visibility: 'shared' },
    })
  ).json();
  const task = await (
    await page.request.post(`/api/v1/workspaces/${wsId}/tasks`, {
      data: { project_id: proj.id, name: `Task Elo ${sufixo}` },
    })
  ).json();
  return { wsId, projectId: proj.id as string, taskId: task.id as string, taskName: `Task Elo ${sufixo}` };
}

async function trackedOf(page: Page, wsId: string, projectId: string, taskId: string): Promise<number> {
  const board = (await (
    await page.request.get(`/api/v1/workspaces/${wsId}/projects/${projectId}/board`)
  ).json()) as { objectives: { tasks: { id: string; tracked_seconds: number }[] }[]; general: { tasks: { id: string; tracked_seconds: number }[] } };
  const all = [...board.objectives.flatMap((o) => o.tasks), ...board.general.tasks];
  return all.find((t) => t.id === taskId)?.tracked_seconds ?? -1;
}

test('CA-16/11/13: Start na task → chip na TimerBar + Parar → o tempo gruda', async ({ page }) => {
  await loginViaStub(page);
  await ensureIdle(page);
  const sufixo = Date.now();
  const { wsId, projectId, taskId, taskName } = await setup(page, sufixo);

  // detalhe → Tasks → abre a task no popup central
  await page.goto(`/projetos/${projectId}`);
  await page.getByRole('tab', { name: /tasks/i }).click();
  await page.getByTestId(`task-card-${taskId}`).click();
  const panel = page.getByTestId('center-panel');
  await expect(panel.getByTestId('task-start')).toHaveAccessibleName('Iniciar');

  // Start na task → chip da task na TimerBar (CA-16) + popup vira Parar (CA-11)
  await panel.getByTestId('task-start').click();
  const bar = page.getByTestId('timerbar');
  await expect(bar.locator('.tt-chip', { hasText: taskName })).toBeVisible();
  await expect(panel.getByTestId('task-start')).toHaveAccessibleName('Parar');

  await page.waitForTimeout(1200); // ≥1s rodando (o /board conta duração ≥ 1s)

  // Parar pelo popup → controle volta pra Start + o tempo grudou na task (CA-13)
  await panel.getByTestId('task-start').click();
  await expect(panel.getByTestId('task-start')).toHaveAccessibleName('Iniciar');
  await expect.poll(() => trackedOf(page, wsId, projectId, taskId)).toBeGreaterThan(0);

  // CA-18 — timer SEM task (label livre pela TimerBar) não mostra chip de task
  await page.getByTestId('center-close').click();
  await bar.getByPlaceholder(/no que você está trabalhando/i).fill(`Livre ${sufixo}`);
  await bar.getByRole('button', { name: /iniciar/i }).click();
  await expect(bar.locator('input')).toHaveValue(`Livre ${sufixo}`); // rodando
  await expect(bar.locator('.tt-chip')).toHaveCount(0); // sem projeto E sem task → nenhum chip
  await ensureIdle(page);
});

test('30-H5: Parar pela TimerBar com o popup FECHADO → o board converge sem reload', async ({ page }) => {
  await loginViaStub(page);
  await ensureIdle(page);
  const sufixo = Date.now();
  const { wsId, projectId, taskId, taskName } = await setup(page, sufixo);

  await page.goto(`/projetos/${projectId}`);
  await page.getByRole('tab', { name: /tasks/i }).click();
  await page.getByTestId(`task-card-${taskId}`).click();
  const panel = page.getByTestId('center-panel');

  // Start pela task → chip na TimerBar
  await panel.getByTestId('task-start').click();
  const bar = page.getByTestId('timerbar');
  await expect(bar.locator('.tt-chip', { hasText: taskName })).toBeVisible();

  // FECHA o popup (o TaskTimeControl desmonta) — o observador do board fica no
  // ProjectDetail (shell sempre montado), não no controle do popup.
  await page.getByTestId('center-close').click();
  await expect(page.getByTestId('center-panel')).toHaveCount(0);

  await page.waitForTimeout(1200); // ≥1s rodando

  // Parar pela TimerBar GLOBAL com o popup fechado → o observador invalida o board
  // (refetch GET /board) SEM reload (30-H5). O waitForResponse prova a invalidação.
  const boardRefetch = page.waitForResponse(
    (r) => /\/projects\/[0-9a-f-]+\/board(\?|$)/.test(r.url()) && r.request().method() === 'GET' && r.ok(),
  );
  await bar.getByRole('button', { name: /parar/i }).click();
  await boardRefetch;

  // o tempo grudou na task (sem reload) — provado pela API do board
  await expect.poll(() => trackedOf(page, wsId, projectId, taskId)).toBeGreaterThan(0);
  await ensureIdle(page);
});
