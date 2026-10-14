import { test, expect, type Page } from '@playwright/test';
import { loginViaStub } from './helpers';
import { acquireTimerLock, releaseTimerLock } from './timerLock';

// T-07 — E2E da fase 27 (Spotlight Cmd+K) contra o app real. CA-19:
//  - Cmd+K abre + foca o input;
//  - /task <nome> + Enter cria a task no projeto de contexto (a task aparece no
//    board — DEGRADAÇÃO D-09/CA-17: NÃO há abridor GLOBAL de popup [TasksTab abre
//    por estado local], então o Spotlight cria e NAVEGA pro projeto; a asserção
//    do "popup com título focado" do tasks.md vira "task no board" — desvio honesto
//    registrado);
//  - sem comando OCIOSO inicia um count-up (CA-14);
//  - sem comando RODANDO edita a descrição do timer SEM zerar o cronômetro
//    (allowance §3.5/CA-13 — E2E-ável, mexe no timer do próprio usuário);
//  - Esc fecha (CA-04).
//
// SERIAL + timerLock: muta o TIMER GLOBAL (count-up/editar), igual cronometro/
// remoto.spec — sob fullyParallel um start/edit clobbera o timer do outro.
test.describe.configure({ mode: 'serial' });
test.beforeAll(acquireTimerLock);
test.afterAll(releaseTimerLock);

async function ensureIdle(page: Page) {
  await page.request.post('/api/v1/timer/stop'); // 409 ocioso = ok
}

// abre o Spotlight pelo atalho e confirma o input focado (CA-01).
async function openSpotlight(page: Page) {
  await page.keyboard.press('ControlOrMeta+KeyK');
  const input = page.getByTestId('spotlight-input');
  await expect(input).toBeFocused();
  return input;
}

test('Cmd+K: /task cria no projeto, count-up ocioso, edita o timer rodando, Esc fecha', async ({ page }) => {
  await loginViaStub(page);
  await ensureIdle(page);

  const sufixo = Date.now();
  const projeto = `Proj Spotlight ${sufixo}`;
  const taskNome = `Comprar domínio ${sufixo}`;

  // cria o projeto e entra no detalhe (rota de contexto /projetos/:id)
  await page.goto('/projetos');
  await page.getByRole('button', { name: /novo projeto/i }).click();
  const proj = page.getByRole('dialog', { name: /novo projeto/i });
  await proj.getByLabel(/nome do projeto/i).fill(projeto);
  await proj.getByRole('button', { name: /criar projeto/i }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await page.getByRole('link', { name: projeto }).click();
  await expect(page).toHaveURL(/\/projetos\/[0-9a-f-]+$/);
  // fase 28 (CA-19): o shell abre na Overview; vai pra Tasks (board)
  await page.getByRole('tab', { name: /tasks/i }).click();
  await expect(page.getByTestId('board-col-todo')).toBeVisible();

  // ── /task <nome> + Enter cria a task; fase 31: abre o popup completo (openTask)
  // já preenchido com o nome (e nasce em BACKLOG). Fecha o popup e confirma a task
  // na coluna Backlog. ──
  let input = await openSpotlight(page);
  await input.fill(`/task ${taskNome}`);
  await page.keyboard.press('Enter');
  await expect(page.getByTestId('spotlight')).toHaveCount(0); // o Spotlight fechou
  // o popup central da task abriu (fase 31), preenchido com o nome
  const taskPanel = page.getByTestId('center-panel');
  await expect(taskPanel).toBeVisible();
  await expect(taskPanel.getByLabel('Título da task')).toHaveValue(taskNome);
  // /task COM nome abre o popup já com o calendário de Due aberto (scheduleOnOpen —
  // "quando fazer?"); o backdrop dele (portal) cobre o ✕. Dispensa clicando fora
  // (canto da tela), depois fecha o popup → a task nasce em Backlog (default da captura).
  await page.mouse.click(5, 5);
  await page.getByTestId('center-close').click();
  await expect(page.getByTestId('center-panel')).toHaveCount(0);
  await expect(page.getByTestId('board-col-backlog').getByText(taskNome)).toBeVisible();

  // ── sem comando, OCIOSO → count-up com a descrição (CA-14) ──
  const descTimer = `Estudar Go ${sufixo}`;
  input = await openSpotlight(page);
  await input.fill(descTimer);
  await page.keyboard.press('Enter');
  await expect(page.getByTestId('spotlight')).toHaveCount(0);
  const bar = page.getByTestId('timerbar');
  const desc = bar.locator('input[readonly]');
  await expect(desc).toHaveValue(descTimer);
  // o cronômetro está correndo
  const digits = bar.locator('.tt-timer-digits');
  await expect(digits).toBeVisible();

  // ── timer RODANDO, sem comando → EDITA a descrição SEM zerar (CA-13) ──
  // deixa o cronômetro acumular um pouco pra provar que não reinicia
  await expect.poll(async () => parseClock(await digits.innerText())).toBeGreaterThanOrEqual(2);
  const antes = parseClock(await digits.innerText());
  const descNova = `Estudar Go avançado ${sufixo}`;
  input = await openSpotlight(page);
  await input.fill(descNova);
  await page.keyboard.press('Enter');
  await expect(page.getByTestId('spotlight')).toHaveCount(0);
  await expect(desc).toHaveValue(descNova); // rótulo atualizou
  // o cronômetro NÃO zerou (segue ≥ ao que estava — allowance não toca timing)
  await expect.poll(async () => parseClock(await digits.innerText())).toBeGreaterThanOrEqual(antes);

  // ── Esc fecha o Spotlight reaberto vazio (CA-04) ──
  await openSpotlight(page);
  await page.keyboard.press('Escape');
  await expect(page.getByTestId('spotlight')).toHaveCount(0);

  // limpa o timer pro próximo run (banco reusado)
  await ensureIdle(page);
});

function parseClock(txt: string): number {
  const p = txt.trim().split(':').map(Number);
  return p.length === 3 ? p[0] * 3600 + p[1] * 60 + p[2] : p[0] * 60 + p[1];
}
