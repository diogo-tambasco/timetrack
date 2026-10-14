import { test, expect } from '@playwright/test';
import { loginViaStub } from './helpers';

// CA-09 + CA-10: rail de navegação, hover-expand 64→232 como overlay sem reflow.
// Fase 18 (sancionado): o rail ganhou "Minhas tasks" → 9 itens.
// Fase 23 (sancionado): o rail ganhou "Remoto" → 10 itens (.tt-nav = 9 navItems
// + Ajustes; a página mudou aditivamente, nenhum item removido).
test.describe('shell do cockpit', () => {
  test('rail visível com os itens de navegação (CA-09)', async ({ page }) => {
    await loginViaStub(page);

    const rail = page.locator('aside.tt-sidenav');
    await expect(rail).toBeVisible();

    const labels = ['Hoje', 'Calendário', 'Projetos', 'Minhas tasks', 'Clientes', 'Relatórios', 'Workspace', 'Foco', 'Remoto', 'Ajustes'];
    const items = rail.locator('.tt-nav');
    await expect(items).toHaveCount(10);
    for (const label of labels) {
      await expect(rail.locator('.tt-navlabel', { hasText: label })).toHaveCount(1);
    }
  });

  test('hover expande pra 232px como overlay sem reflow (CA-10)', async ({ page }) => {
    await loginViaStub(page);

    const rail = page.locator('aside.tt-sidenav');
    const inner = page.locator('.tt-sidenav-inner');
    const main = page.locator('main');

    // repouso: 64px reservados
    const railBox = await rail.boundingBox();
    expect(railBox!.width).toBe(64);
    const mainXBefore = (await main.boundingBox())!.x;

    // hover → inner vira overlay de 232px
    await rail.hover();
    await expect.poll(async () => (await inner.boundingBox())!.width).toBe(232);

    // sem reflow: a largura reservada segue 64px e o conteúdo não se move
    expect((await rail.boundingBox())!.width).toBe(64);
    expect((await main.boundingBox())!.x).toBe(mainXBefore);

    // sai do hover → recolhe pra 64px
    await page.mouse.move(600, 400);
    await expect.poll(async () => (await inner.boundingBox())!.width).toBe(64);
  });
});
