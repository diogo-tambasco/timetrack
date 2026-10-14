import { test, expect } from '@playwright/test';
import { loginViaStub } from './helpers';

// Fase 24 T-16 (CA-28): o fluxo da aba Tasks 2.0 ponta a ponta — logado (stub) com
// escrita num projeto shared, em /projetos/{id}: (a) Board com 4 colunas; (b)
// quick-add numa coluna (backlog) → task aparece nela + popup central abre; (c)
// editar o título; (d) sub-task pela sidebar + ⭐ + feita; (e) link do Notion →
// preview; (f) expandir o popup pra tela cheia e voltar; (g) fechar → Table → a
// task aparece; (h) reload persiste. Sem perfil viewer (stub = 1 user — CA-28); o
// DnD/gating são testes de componente jsdom.
test('aba Tasks 2.0: quick-add, popup, sub-task, notion, expand, table, persiste', async ({ page }) => {
  await loginViaStub(page);

  const sufixo = Date.now();
  const projeto = `Proj Tasks2 ${sufixo}`;
  const titulo = `Investigar churn ${sufixo}`;

  // cria o projeto
  await page.goto('/projetos');
  await page.getByRole('button', { name: /novo projeto/i }).click();
  const proj = page.getByRole('dialog', { name: /novo projeto/i });
  await proj.getByLabel(/nome do projeto/i).fill(projeto);
  await proj.getByRole('button', { name: /criar projeto/i }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);

  // abre o detalhe → tab-shell abre na Overview (fase 28, CA-19); vai pra Tasks
  await page.getByRole('link', { name: projeto }).click();
  await expect(page).toHaveURL(/\/projetos\/[0-9a-f-]+$/);
  await page.getByRole('tab', { name: /tasks/i }).click();
  await expect(page.getByTestId('board-col-backlog')).toBeVisible();
  await expect(page.getByTestId('board-col-done')).toBeVisible();

  // (b) quick-add na coluna Backlog → cria "New task" e abre o popup focado
  await page.getByRole('button', { name: /add em backlog/i }).click();
  const panel = page.getByTestId('center-panel');
  await expect(panel).toBeVisible();
  const title = panel.getByLabel('Título da task');
  await expect(title).toHaveValue('New task');

  // (c) edita o título (Enter salva e foca a descrição — CA-21)
  await title.fill(titulo);
  await title.press('Enter');
  await expect(panel.getByLabel('Descrição da task')).toBeFocused();

  // (d) task-goal pela sidebar + concluir (fase 31: removida a estrela is_goal —
  // todo item já é um goal da task; o label do add virou "Adicionar task-goal")
  const addSub = panel.getByLabel('Adicionar task-goal');
  await addSub.fill('Revisar com o Carlos');
  await addSub.press('Enter');
  await expect(panel.getByText('Revisar com o Carlos')).toBeVisible();
  await panel.getByLabel('Concluir Revisar com o Carlos').click();
  // cabeçalho reflete 1/1 feita
  await expect(panel.getByTestId('subtask-sidebar')).toContainText('1/1');

  // (e) cola um link do Notion → preview
  const notionInput = panel.getByLabel('Link do Notion');
  await notionInput.fill('https://www.notion.so/Churn-8F3A1C9B');
  await notionInput.press('Enter');
  await expect(panel.getByTestId('notion-preview')).toBeVisible();
  await expect(panel.getByText('Churn')).toBeVisible();

  // (f) expande pra tela cheia e volta
  await page.getByTestId('center-toggle').click();
  await expect(page.getByTestId('center-card')).toHaveAttribute('data-mode', 'full');
  await page.getByTestId('center-toggle').click();
  await expect(page.getByTestId('center-card')).toHaveAttribute('data-mode', 'side');

  // (g) fecha → Table → a task aparece
  await page.getByTestId('center-close').click();
  await expect(page.getByTestId('center-panel')).toHaveCount(0);
  await page.getByRole('tab', { name: /table/i }).click();
  await expect(page.getByTestId('task-table')).toBeVisible();
  // o nome da task é um botão (abre o popup); status/priority são chips com o nome
  // no aria-label — busca o botão exato pra não casar múltiplas células.
  await expect(page.getByTestId('task-table').getByRole('button', { name: titulo, exact: true })).toBeVisible();

  // (h) reload persiste a VISÃO Table (CA-16.1/RF-03): a Table nativa grava o token
  // próprio 'table-v2' em tt.view.board (não colide com o 'table' legado da fase 12)
  // → ao recarregar, a Table reaparece SEM re-clicar e a task (priority/due/time)
  // segue lá.
  await page.reload();
  // fase 28 (CA-19): o reload reabre na Overview; volta pra Tasks. A visão Table
  // persiste (token table-v2 / 24-H1) → reabre na Table sem re-clicar.
  await page.getByRole('tab', { name: /tasks/i }).click();
  await expect(page.getByTestId('task-table')).toBeVisible();
  await expect(page.getByTestId('task-table').getByRole('button', { name: titulo, exact: true })).toBeVisible();
});
