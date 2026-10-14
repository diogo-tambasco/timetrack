import { test, expect } from '@playwright/test';
import { loginViaStub } from './helpers';

// CA-11 (E2E): dark default na primeira visita. CA-12: light opcional e
// persistente entre reloads via toggle em Ajustes (que nasce nesta T-15).
test('tema dark default; toggle pra light persiste após reload (CA-11/CA-12)', async ({ page }) => {
  await loginViaStub(page);

  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  const bg = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
  expect(bg).toBe('rgb(14, 14, 16)'); // #0e0e10

  await page.goto('/ajustes');
  // fase 14: o /ajustes ganhou o grupo Relatórios com um switch (workdays_only)
  // — escopa o toggle ao switch do TEMA por aria-label (desvio mecânico
  // sancionado; espelha o escopo do cronometro.spec na fase 11). A fase 15
  // adiciona mais switches no Pomodoro — este escopo já fica durável.
  await page.getByRole('switch', { name: /tema claro/i }).click();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');

  await page.reload();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
});
