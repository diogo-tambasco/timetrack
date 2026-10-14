import { test, expect } from '@playwright/test';
import { loginViaStub } from './helpers';

// Fase 26 T-12 (CA-23): o fluxo das Notas ponta a ponta — logado (stub) com
// escrita num projeto shared, em /projetos/{id}: (a) abre a aba Notes; (b) New
// note → editor abre; (c) digita título + corpo md; (d) / + Task → task ligada
// (chip no topo, menu some); (e) fecha o editor → a nota aparece na lista com
// preview; (f) aba Tasks → a task criada existe; (g) reabre a nota → corpo/chip
// persistem após reload. Sem perfil viewer (stub = 1 user — CA-23); o gating é a
// T-11 (jsdom). Nomes únicos (banco reusado, idempotente).
test('aba Notes: New note, editor, / + Task, lista com preview, persiste', async ({ page }) => {
  await loginViaStub(page);

  const sufixo = Date.now();
  const projeto = `Proj Notas ${sufixo}`;
  const tituloNota = `Decisões ${sufixo}`;

  // cria o projeto
  await page.goto('/projetos');
  await page.getByRole('button', { name: /novo projeto/i }).click();
  const proj = page.getByRole('dialog', { name: /novo projeto/i });
  await proj.getByLabel(/nome do projeto/i).fill(projeto);
  await proj.getByRole('button', { name: /criar projeto/i }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);

  // abre o detalhe → vai pra aba Notes
  await page.getByRole('link', { name: projeto }).click();
  await expect(page).toHaveURL(/\/projetos\/[0-9a-f-]+$/);
  await page.getByRole('tab', { name: /notes/i }).click();
  await expect(page.getByTestId('notes-empty')).toBeVisible();

  // (b) New note → editor abre no CenterPanel da 24
  await page.getByTestId('new-note').click();
  const panel = page.getByTestId('center-panel');
  await expect(panel).toBeVisible();

  // (c) digita título + corpo markdown
  await panel.getByTestId('note-title').fill(tituloNota);
  const body = panel.getByTestId('note-body');
  await body.click();
  await body.fill('## Resumo\nCobrança anual vira default.');

  // (d) / + Task → cria a task ligada (chip no topo, menu some). A barra abre o
  // menu; Enter confirma o 1º item (Task).
  await body.fill('## Resumo\nCobrança anual vira default.\n/task');
  await expect(panel.getByTestId('slash-menu')).toBeVisible();
  // garante que o 1º item é Task e confirma
  await body.press('Enter');
  await expect(panel.getByTestId('slash-menu')).toHaveCount(0);
  await expect(panel.getByTestId('note-meta')).toBeVisible();
  await expect(panel.getByTestId('note-chip-task')).toBeVisible();

  // (e) fecha o editor → a nota aparece na lista com preview
  await page.getByTestId('center-close').click();
  await expect(page.getByTestId('center-panel')).toHaveCount(0);
  await expect(page.getByTestId('notes-table')).toBeVisible();
  await expect(page.getByText(tituloNota)).toBeVisible();
  // o preview tem o texto do corpo (markdown removido — "Resumo").
  await expect(page.getByTestId('notes-table')).toContainText('Resumo');

  // (f) aba Tasks → a task criada pelo slash existe (status todo)
  await page.getByRole('tab', { name: /tasks/i }).click();
  await expect(page.getByTestId('board-col-todo')).toContainText('New task');

  // (g) volta pra Notes, reabre a nota → corpo/chip persistem após reload
  await page.reload();
  await page.getByRole('tab', { name: /notes/i }).click();
  await page.getByText(tituloNota).click();
  const panel2 = page.getByTestId('center-panel');
  await expect(panel2).toBeVisible();
  await expect(panel2.getByTestId('note-chip-task')).toBeVisible();
  await expect(panel2.getByTestId('note-body')).toHaveValue(/Resumo/);
});
