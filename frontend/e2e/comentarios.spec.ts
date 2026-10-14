import { test, expect } from '@playwright/test';
import { loginViaStub } from './helpers';

// Fase 24 T-16 (reconcilia o comentarios.spec da fase 19 — CA-28.1): a thread de
// comentários da fase 19 agora aparece no POPUP central (TaskPanel), não mais no
// dialog "Editar task" (TaskModal aposentado do board). Logado (stub) como owner:
// cria projeto → quick-add abre o popup → comenta → ordem ASC → apaga o próprio →
// reload reabre e persiste. Só owner/member (stub = 1 user); o carve-out
// viewer-assignee é jsdom (canComment.spec) + Go (comments_handlers_test).
test('thread no popup: comentar, ordem ASC, apagar o próprio e persistir no reload', async ({ page }) => {
  await loginViaStub(page);

  const sufixo = Date.now();
  const projeto = `Proj Comentários ${sufixo}`;
  const task = `Task comentável ${sufixo}`;

  // cria o projeto
  await page.goto('/projetos');
  await page.getByRole('button', { name: /novo projeto/i }).click();
  const proj = page.getByRole('dialog', { name: /novo projeto/i });
  await proj.getByLabel(/nome do projeto/i).fill(projeto);
  await proj.getByRole('button', { name: /criar projeto/i }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);

  // abre o detalhe → tab-shell abre na Overview (fase 28, CA-19); vai pra Tasks →
  // quick-add abre o popup; renomeia
  await page.getByRole('link', { name: projeto }).click();
  await expect(page).toHaveURL(/\/projetos\/[0-9a-f-]+$/);
  await page.getByRole('tab', { name: /tasks/i }).click();
  await page.getByRole('button', { name: /add em to do/i }).click();
  const panel = page.getByTestId('center-panel');
  const title = panel.getByLabel('Título da task');
  await title.fill(task);
  await title.blur();

  // a thread vive no popup
  await expect(panel.getByRole('region', { name: 'Comentários', exact: true })).toBeVisible();
  await expect(panel.getByText(/nenhum comentário ainda/i)).toBeVisible();

  // 1º comentário
  await panel.getByLabel(/novo comentário/i).fill('primeiro comentário');
  await panel.getByRole('button', { name: /comentar/i }).click();
  const c1 = panel.getByTestId('comment').filter({ hasText: 'primeiro comentário' });
  await expect(c1).toBeVisible();
  await expect(c1).toContainText(/diogo/i);

  // 2º comentário aparece ABAIXO (ASC)
  await panel.getByLabel(/novo comentário/i).fill('segundo comentário');
  await panel.getByRole('button', { name: /comentar/i }).click();
  await expect(panel.getByTestId('comment')).toHaveCount(2);
  const itens = panel.getByTestId('comment');
  await expect(itens.nth(0)).toContainText('primeiro comentário');
  await expect(itens.nth(1)).toContainText('segundo comentário');

  // apaga o PRÓPRIO 1º → some
  await c1.getByRole('button', { name: /apagar comentário/i }).click();
  await expect(panel.getByText('primeiro comentário')).toHaveCount(0);
  await expect(panel.getByTestId('comment')).toHaveCount(1);

  // reload + reabrir a task (Tasks → Table → clicar) → persistiu só o 2º.
  // fase 28 (CA-19): o reload reabre na Overview; volta pra Tasks antes da Table.
  await page.reload();
  await page.getByRole('tab', { name: /tasks/i }).click();
  await page.getByRole('tab', { name: /table/i }).click();
  await page.getByTestId('task-table').getByRole('button', { name: task, exact: true }).click();
  const panel2 = page.getByTestId('center-panel');
  await expect(panel2.getByTestId('comment')).toHaveCount(1);
  await expect(panel2.getByText('segundo comentário')).toBeVisible();
  await expect(panel2.getByText('primeiro comentário')).toHaveCount(0);
});
