import { test, expect } from '@playwright/test';
import { loginViaStub } from './helpers';

// CA-17a (T-14): não autenticado nunca vê o app; autenticado nunca vê
// /login; sair revoga de verdade (reusa CA-11/CA-13 ponta a ponta).
test.describe('guard de autenticação', () => {
  test('sem sessão, toda rota (exceto /login) redireciona pra /login, sem loop', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveURL(/\/login$/);

    await page.goto('/projetos');
    await expect(page).toHaveURL(/\/login$/);

    // /login em si não entra em loop e renderiza o CTA
    await page.goto('/login');
    await expect(page).toHaveURL(/\/login$/);
    await expect(page.getByRole('link', { name: /entrar com google/i })).toBeVisible();
  });

  test('login stubbado cai no cockpit e /me responde 200', async ({ page }) => {
    await loginViaStub(page);

    // cockpit de pé (rota / do shell da fase 00)
    await expect(page.locator('aside.tt-sidenav')).toBeVisible();

    // a sessão vale pra API (CA-10/CA-11 ponta a ponta)
    const res = await page.request.get('/api/v1/me');
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.workspaces[0].role).toBe('owner');
  });

  test('sair volta pro /login e a sessão morre de verdade', async ({ page }) => {
    await loginViaStub(page);

    await page.getByRole('button', { name: 'Conta' }).click();
    await page.getByRole('button', { name: 'Sair' }).click();
    await expect(page).toHaveURL(/\/login$/);

    // sessão revogada no servidor (CA-13): API nega…
    const res = await page.request.get('/api/v1/me');
    expect(res.status()).toBe(401);

    // …e o app volta a redirecionar
    await page.goto('/');
    await expect(page).toHaveURL(/\/login$/);
  });
});
