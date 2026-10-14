import { test, expect } from '@playwright/test';
import { loginViaStub, seedWorkspaceMember } from './helpers';

// T-21 — smoke E2E do problema central nº1 (parcial, sem timer — TDD.md
// §4.1): login stub → cria cliente → cria projeto compartilhado com
// membro "Só vê" → abre o board do projeto. A gestão de tasks migrou pro
// board (fase 12 T-12, ProjectTasks inline aposentado); o fluxo completo de
// objetivos/tasks está em tarefas.spec.
test('fluxo central: cliente → projeto compartilhado com membro view → abre o board', async ({
  page,
}) => {
  await loginViaStub(page);
  seedWorkspaceMember({
    googleId: 'g-e2e-bruno',
    name: 'Bruno E2E',
    email: 'bruno.e2e@example.com',
    role: 'member',
  });

  const sufixo = Date.now();
  const cliente = `Cliente E2E ${sufixo}`;
  const projeto = `Projeto Central ${sufixo}`;

  // 1) cria o cliente
  await page.goto('/clientes');
  await page.getByRole('button', { name: /novo cliente/i }).click();
  await page.getByLabel(/nome do cliente/i).fill(cliente);
  await page.getByRole('button', { name: /^criar$/i }).click();
  await expect(page.getByText(cliente)).toBeVisible();

  // 2) cria o projeto compartilhado, com cliente e membro "Só vê"
  await page.goto('/projetos');
  await page.getByRole('button', { name: /novo projeto/i }).click();
  const dialog = page.getByRole('dialog', { name: /novo projeto/i });
  await dialog.getByLabel(/nome do projeto/i).fill(projeto);
  await dialog.getByLabel(/^cliente$/i).selectOption({ label: cliente });
  await dialog.getByLabel(/incluir bruno e2e/i).click();
  await dialog.getByRole('button', { name: /só vê/i }).click();
  await dialog.getByRole('button', { name: /criar projeto/i }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);

  // o cartão novo lista projeto + cliente certos, sem reload
  const card = page.locator('.tt-card', { hasText: projeto });
  await expect(card).toBeVisible();
  await expect(card).toContainText(cliente);

  // o membro persistiu com o acesso escolhido: reabrir em Editar prova
  await card.getByRole('button', { name: /^editar$/i }).click();
  const edit = page.getByRole('dialog', { name: /editar projeto/i });
  await expect(edit.getByLabel(/incluir bruno e2e/i)).toHaveAttribute('aria-pressed', 'true');
  await expect(edit.getByRole('button', { name: /só vê/i })).toHaveAttribute(
    'aria-pressed',
    'true',
  );
  await edit.getByRole('button', { name: /cancelar/i }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);

  // 3) o cartão abre o WORKSPACE DE PROJETO (detalhe /projetos/:id) — fase 24
  // (CA-28.1): o board simples da fase 12 virou tab-shell (Overview/Tasks/Notes/
  // Goals/Calendar). Fase 28 (CA-19): a aba default é Overview; aqui confirmamos
  // que o shell abre e que a aba Tasks (board de 4 colunas) abre ao clicar. O
  // ciclo de task está em tarefas.spec.
  await card.getByRole('link', { name: projeto }).click();
  await expect(page).toHaveURL(/\/projetos\/[0-9a-f-]+$/);
  await expect(page.getByRole('heading', { name: projeto })).toBeVisible();
  await expect(page.getByRole('tab', { name: 'Tasks' })).toBeVisible();
  await page.getByRole('tab', { name: 'Tasks' }).click();
  await expect(page.getByTestId('board-col-todo')).toBeVisible();
});
