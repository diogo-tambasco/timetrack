// projeto-vivo.spec — fase 28 T-13 (CA-22): projeto vivo + calendário do projeto
// ponta a ponta. Logado (stub) num projeto SHARED com escrita: deadline + task
// com due HOJE + entry fechada (via seed SQL — não há UI de editar deadline). O
// teste vê os CARDS VIVOS (horas reais ≠ 0h, "X pendentes", dot "hoje"); abre o
// projeto → aba Overview (KPIs, Upcoming com a task de prazo, Progress); abre a
// aba Calendar → conecta o GCal (calendarstub) → vê o item de PRAZO do projeto e
// o DUE da task + ≥1 evento do GCal sobreposto; CLICA no item da task → abre a
// task; navega de mês → a grade muda. SEM perfil viewer (stub = 1 user — CA-22:
// o gating de viewer é coberto pelo jsdom da T-12).
//
// Exceção ao red estrutural (precedente das fases 06/11/24): o app é entregue por
// T-01..T-12; red honesto = spec rodado ANTES do wiring das abas.
import { test, expect } from '@playwright/test';
import { loginViaStub, seedProjectVivo, resetCalendarConnection } from './helpers';

test('CA-22: cards vivos → Overview → Calendar (itens do projeto + GCal) → abrir task → navegar mês', async ({
  page,
}) => {
  await loginViaStub(page);

  const sufixo = Date.now();
  const projeto = `Proj Vivo ${sufixo}`;
  const task = `Fechar OKRs ${sufixo}`;
  seedProjectVivo({ project: projeto, task });
  // db-test persistente + user único: começa SEMPRE desconectado do GCal.
  resetCalendarConnection();

  // (1) lista de Projetos: card VIVO — horas reais (≠ 0h), "1 pendentes", dot "hoje"
  await page.goto('/projetos');
  const card = page.locator('.tt-card', { hasText: projeto });
  await expect(card).toBeVisible();
  await expect(card.getByText(/1 pendente · 1 no total/)).toBeVisible();
  await expect(card.getByText('1h')).toBeVisible(); // 3600s = 1h, ≠ 0h hardcoded
  await expect(card.getByTestId('project-today')).toBeVisible(); // due HOJE
  await expect(card.getByTestId('project-deadline')).toBeVisible(); // prazo do projeto

  // (2) abre o projeto → aba Overview (default) com KPIs + Upcoming + Progress
  await card.getByRole('link', { name: projeto }).click();
  await expect(page).toHaveURL(/\/projetos\/[0-9a-f-]+$/);
  await expect(page.getByRole('tab', { name: /overview/i })).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByText('Total time')).toBeVisible();
  await expect(page.getByText('Upcoming')).toBeVisible();
  // Upcoming traz a task de prazo (due hoje, status != done)
  await expect(page.getByText(task)).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Progress' })).toBeVisible();

  // (3) aba Calendar → grade do mês com o PRAZO do projeto e o DUE da task
  await page.getByRole('tab', { name: /calendar/i }).click();
  await expect(page.getByTestId('project-month-grid')).toBeVisible();
  // o item da task aparece (due no mês corrente)
  const itemTask = page.getByTestId('cal-item').filter({ hasText: task });
  await expect(itemTask).toBeVisible();
  // o prazo do projeto também é um item (nome do projeto)
  await expect(page.getByTestId('cal-item').filter({ hasText: projeto })).toBeVisible();

  // (4) conecta o GCal → o overlay sobrepõe ≥1 evento do calendarstub
  await page.goto('/calendario');
  await page.getByTestId('connect-calendar').click();
  await expect(page).toHaveURL(/\/calendario$/);
  // volta pro projeto, aba Calendar → o evento do GCal aparece sobreposto (CA-13)
  await page.goto('/projetos');
  await page.locator('.tt-card', { hasText: projeto }).getByRole('link', { name: projeto }).click();
  await page.getByRole('tab', { name: /calendar/i }).click();
  await expect(page.getByTestId('project-month-grid')).toBeVisible();
  await expect(page.getByTestId('cal-event').filter({ hasText: 'Reunião do dia' }).first()).toBeVisible();

  // (5) clica no item da task → abre a task (TaskPanel — RF-23)
  await page.getByTestId('cal-item').filter({ hasText: task }).click();
  await expect(page.getByTestId('center-panel')).toBeVisible();
  await expect(page.getByLabel('Título da task')).toHaveValue(task);
  await page.getByTestId('center-close').click();
  await expect(page.getByTestId('center-panel')).toHaveCount(0);

  // (6) navega de mês (próximo) → a grade muda (o item da task some no próximo mês)
  const grid = page.getByTestId('project-month-grid');
  await page.getByRole('button', { name: /próximo/i }).click();
  await expect(grid).toBeVisible();
  await expect(page.getByTestId('cal-item').filter({ hasText: task })).toHaveCount(0);
});
