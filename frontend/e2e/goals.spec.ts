// goals.spec — fase 29 T-12 (CA-16): a aba Goals ponta a ponta. Logado (stub) com
// escrita: cria um Goal POR HORAS (vê a linha com a coluna de horas /meta) e um
// Goal POR DATA (by_date) que nasce Open; FECHA o by_date MANUALMENTE pelo chip
// (CA-08 — by_date não tem auto-close, CA-07) e o vê Done; recarrega e o modo/
// estado PERSISTE. SEM perfil viewer (stub = 1 user): o gating (CA-15) é jsdom
// (GoalsTab.spec). O auto-close POR HORAS (CA-06) e o no-op do by_date com tasks
// done (CA-07) são teste Go (objective_status_test) — precisam semear entries/
// transições de task, inviável no E2E sem cronômetro real.
import { test, expect } from '@playwright/test';
import { loginViaStub } from './helpers';

test('CA-16: cria goal by_hours e by_date → fecha o by_date manual → Done → persiste', async ({
  page,
}) => {
  await loginViaStub(page);

  const sufixo = Date.now();
  const projeto = `Proj Goals ${sufixo}`;
  const goalHoras = `Bater 40h ${sufixo}`;
  const goalData = `Demo cliente ${sufixo}`;

  // cria o projeto
  await page.goto('/projetos');
  await page.getByRole('button', { name: /novo projeto/i }).click();
  const proj = page.getByRole('dialog', { name: /novo projeto/i });
  await proj.getByLabel(/nome do projeto/i).fill(projeto);
  await proj.getByRole('button', { name: /criar projeto/i }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);

  // abre o projeto → aba Goals (default = Overview, fase 28)
  await page.getByRole('link', { name: projeto }).click();
  await expect(page).toHaveURL(/\/projetos\/[0-9a-f-]+$/);
  await page.getByRole('tab', { name: /goals/i }).click();
  await expect(page.getByText(/No goals yet\./i)).toBeVisible();

  // (1) New goal POR HORAS com meta de 40h
  await page.getByRole('button', { name: /new goal/i }).click();
  let modal = page.getByRole('dialog', { name: /novo goal/i });
  await modal.getByLabel(/nome do objetivo/i).fill(goalHoras);
  await modal.getByRole('button', { name: /por horas/i }).click();
  await modal.getByLabel(/meta de horas/i).fill('40');
  await modal.getByRole('button', { name: /criar goal/i }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);

  // a linha do goal aparece com a coluna de horas /40h (barra por modo)
  const linhaHoras = page.getByTestId('goal-row').filter({ hasText: goalHoras });
  await expect(linhaHoras).toBeVisible();
  await expect(linhaHoras.getByText(/\/40h/)).toBeVisible();

  // (2) New goal POR DATA (by_date) com marco → nasce Open
  await page.getByRole('button', { name: /new goal/i }).click();
  modal = page.getByRole('dialog', { name: /novo goal/i });
  await modal.getByLabel(/nome do objetivo/i).fill(goalData);
  await modal.getByRole('button', { name: /por data/i }).click();
  await modal.getByLabel(/data do marco/i).fill('2026-12-15');
  await modal.getByRole('button', { name: /criar goal/i }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);

  const linhaData = page.getByTestId('goal-row').filter({ hasText: goalData });
  await expect(linhaData).toBeVisible();
  await expect(linhaData.getByText('Open')).toBeVisible();

  // (3) fecha o by_date MANUALMENTE pelo chip (CA-08) → Done (auto-close não fecha
  // by_date — CA-07)
  await linhaData.getByRole('button', { name: /concluir goal/i }).click();
  await expect(linhaData.getByText('Done')).toBeVisible();

  // (4) reload → aba Goals → o by_date segue Done e o by_hours segue listado (modo
  // e estado persistem no banco)
  await page.reload();
  await page.getByRole('tab', { name: /goals/i }).click();
  const linhaDataR = page.getByTestId('goal-row').filter({ hasText: goalData });
  await expect(linhaDataR.getByText('Done')).toBeVisible();
  await expect(page.getByTestId('goal-row').filter({ hasText: goalHoras })).toBeVisible();
});
