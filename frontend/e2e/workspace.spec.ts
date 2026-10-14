// T-04 da fase 10 (CA-10) — página Workspace + seletor persistido.
// Arquivo PARALELO (não muta timer). Seeds DEPOIS do login + reload — a
// letra do CA-10 e o precedente do core-projetos.spec (verificação
// adversarial do build: semear antes do 1º login em banco FRESCO criava o
// 2º workspace ÓRFÃO com created_at ANTERIOR ao pessoal e envenenava o
// workspaces[0] da suíte inteira até o restart do tmpfs).
//
// Exceção ao red estrutural documentada (precedente do T-19 da 03): o app
// é entregue por T-01..T-03; red honesto = spec rodado ANTES de ajuste.
import { test, expect } from '@playwright/test';
import { loginViaStub, seedWorkspaceMember, seedSecondWorkspace } from './helpers';

test('CA-10: nome + membros com roles + seletor de workspace ativo persistido', async ({
  page,
}) => {
  await loginViaStub(page);
  await page.goto('/workspace');

  // nome do pessoal + o próprio usuário como Dono
  await expect(page.getByTestId('workspace-name')).toHaveText('Workspace de Diogo');
  const linhaDono = page.locator('[data-testid^="member-"]', { hasText: 'Diogo Fixture' });
  await expect(linhaDono).toContainText('Dono');

  // seeds DEPOIS do login (created_at do pessoal < secundário SEMPRE) +
  // reload — o TanStack não refaz a busca sozinho
  seedWorkspaceMember({
    googleId: 'g-ws-extra',
    name: 'Carlos Workspace',
    email: 'carlos.ws@example.com',
    role: 'member',
  });
  seedSecondWorkspace('Workspace Secundário E2E');
  await page.reload();

  // membro semeado por SQL aparece com a role certa
  const linhaCarlos = page.locator('[data-testid^="member-"]', { hasText: 'Carlos Workspace' });
  await expect(linhaCarlos).toContainText('Membro');

  // seletor presente (2 workspaces no /me) → trocar muda o contexto
  const seletor = page.getByRole('combobox', { name: /workspace ativo/i });
  await expect(seletor).toBeVisible();
  await seletor.selectOption({ label: 'Workspace Secundário E2E' });
  await expect(page.getByTestId('workspace-name')).toHaveText('Workspace Secundário E2E');

  // persiste após reload (localStorage tt.ws.active)
  await page.reload();
  await expect(page.getByTestId('workspace-name')).toHaveText('Workspace Secundário E2E');

  // volta pro pessoal — contexto limpo pro retry do próprio runner (plan §6)
  await page
    .getByRole('combobox', { name: /workspace ativo/i })
    .selectOption({ label: 'Workspace de Diogo' });
  await expect(page.getByTestId('workspace-name')).toHaveText('Workspace de Diogo');
});
