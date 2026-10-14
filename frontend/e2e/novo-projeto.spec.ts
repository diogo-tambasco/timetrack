import { test, expect } from '@playwright/test';
import { loginViaStub } from './helpers';

// CA-29 (T-19): confirmar o modal Novo projeto com dados válidos faz o
// projeto aparecer na lista SEM recarregar a página, e o modal fecha.
test('criar projeto pelo modal reflete na lista sem reload e fecha o modal', async ({ page }) => {
  await loginViaStub(page);
  await page.goto('/projetos');
  await expect(page.getByRole('button', { name: /novo projeto/i })).toBeVisible();

  // sentinela no window: um reload da página a apagaria
  await page.evaluate(() => {
    (window as unknown as Record<string, boolean>).__ttSemReload = true;
  });

  await page.getByRole('button', { name: /novo projeto/i }).click();
  const dialog = page.getByRole('dialog', { name: /novo projeto/i });
  await expect(dialog).toBeVisible();

  const nome = `Projeto E2E ${Date.now()}`;
  await dialog.getByLabel(/nome do projeto/i).fill(nome);
  await dialog.getByRole('button', { name: /criar projeto/i }).click();

  // o modal fecha e o projeto aparece na lista
  await expect(page.getByRole('dialog')).toHaveCount(0);
  // getByRole(main): desde a fase 03 o projeto novo também vira <option>
  // no select da TimerBar — o alvo da asserção é a LISTA da página
  await expect(page.getByRole('main').getByText(nome)).toBeVisible();

  // a sentinela sobreviveu → não houve reload (CA-29)
  expect(
    await page.evaluate(() => (window as unknown as Record<string, boolean>).__ttSemReload),
  ).toBe(true);
});
