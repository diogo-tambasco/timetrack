import { test, expect, type Page } from '@playwright/test';
import { loginViaStub } from './helpers';

// Fase 22 T-10 (CA-19/20) — E2E mobile. FONTE DE VERDADE do "zero overflow a
// 375px" (Vitest+jsdom não mede scrollWidth — FURO 1). Logado (stub), numa
// viewport 375×667, varre TODAS as rotas-chave (incl. as densas: board,
// clientes, calendário, workspace) e afirma, em CADA: (a) o documento NÃO tem
// overflow horizontal; (b) a nav mobile está presente (exceto /foco, overlay
// fullscreen — CA-16). Ação principal alcançável (abrir "Nova entrada"). Um
// describe de CONTROLE desktop (1280×800) prova a inversão: nav mobile ausente,
// rail presente. Não muta o timer global (parte parável — só navega/abre modal).

// scrollWidth <= clientWidth do documento = zero overflow horizontal.
async function semOverflowHorizontal(page: Page): Promise<boolean> {
  return page.evaluate(
    () =>
      document.documentElement.scrollWidth <= document.documentElement.clientWidth,
  );
}

test.describe('mobile 375×667 — zero overflow + nav mobile', () => {
  test.use({ viewport: { width: 375, height: 667 } });

  test('varre as rotas-chave sem overflow horizontal; nav mobile presente', async ({
    page,
  }) => {
    await loginViaStub(page);

    // cria um projeto pra alcançar o board /projetos/:id (padrão tarefas.spec)
    const projeto = `Proj Mobile ${Date.now()}`;
    await page.goto('/projetos');
    await page.getByRole('button', { name: /novo projeto/i }).click();
    const proj = page.getByRole('dialog', { name: /novo projeto/i });
    await proj.getByLabel(/nome do projeto/i).fill(projeto);
    await proj.getByRole('button', { name: /criar projeto/i }).click();
    await expect(page.getByRole('dialog')).toHaveCount(0);
    await page.getByRole('link', { name: projeto }).click();
    await expect(page).toHaveURL(/\/projetos\/[0-9a-f-]+$/);
    const boardUrl = page.url();

    // todas as rotas-chave (CA-19). /foco é overlay fullscreen (CA-16) — só o
    // overflow se aplica lá, a bottom bar não.
    const rotas: { url: string; nav: boolean; label: string }[] = [
      { url: '/', nav: true, label: 'Dashboard' },
      { url: '/projetos', nav: true, label: 'Projetos' },
      { url: boardUrl, nav: true, label: 'Board (projetos/:id)' },
      { url: '/clientes', nav: true, label: 'Clientes' },
      { url: '/relatorios', nav: true, label: 'Relatórios' },
      { url: '/calendario', nav: true, label: 'Calendário' },
      { url: '/workspace', nav: true, label: 'Workspace' },
      { url: '/ajustes', nav: true, label: 'Ajustes' },
      { url: '/foco', nav: false, label: 'Foco' },
    ];

    for (const r of rotas) {
      await page.goto(r.url);
      // dá um tempo do conteúdo assentar
      await page.waitForLoadState('networkidle');
      expect(await semOverflowHorizontal(page), `overflow em ${r.label}`).toBe(true);
      if (r.nav) {
        await expect(
          page.getByTestId('mobile-nav'),
          `nav mobile em ${r.label}`,
        ).toBeVisible();
      }
    }
  });

  test('a nav mobile navega pelas rotas-chave (URL muda)', async ({ page }) => {
    await loginViaStub(page);
    await page.goto('/');
    const nav = page.getByTestId('mobile-nav');
    await expect(nav).toBeVisible();

    // toca em Projetos pela bottom bar → vai pra /projetos
    await nav.getByRole('link', { name: /projetos/i }).click();
    await expect(page).toHaveURL(/\/projetos$/);

    // toca em Relatórios
    await nav.getByRole('link', { name: /relatórios/i }).click();
    await expect(page).toHaveURL(/\/relatorios$/);
  });

  test('o <main> tem respiro inferior >= altura da bottom bar (plan §7 — bar não cobre o conteúdo)', async ({
    page,
  }) => {
    await loginViaStub(page);
    await page.goto('/');
    await page.waitForLoadState('networkidle');
    // abaixo de md o padding-bottom resolve pra calc(--mobnav-h + safe-area +
    // --pad) > 58px (altura da bar fixa). ANTES do fix o shorthand inline
    // `padding:var(--pad)` derrotava a regra .tt-mainpad → 18px e a bar cobria
    // o último elemento. (jsdom não computa @media+classe → tem de ser E2E.)
    const padBottom = await page.evaluate(() => {
      const main = document.querySelector('main.tt-mainpad');
      return main ? parseFloat(getComputedStyle(main).paddingBottom) : -1;
    });
    expect(padBottom).toBeGreaterThanOrEqual(58);
  });

  test('ação principal alcançável: "Nova entrada" abre sem estourar (CA-20)', async ({
    page,
  }) => {
    await loginViaStub(page);
    await page.goto('/');
    await page.getByRole('button', { name: /nova entrada/i }).click();
    await expect(page.getByRole('dialog', { name: /nova entrada/i })).toBeVisible();
    // o modal aberto não estoura o documento a 375px
    expect(await semOverflowHorizontal(page)).toBe(true);
  });
});

test.describe('controle desktop 1280×800 — inversão (CA-13/20)', () => {
  test.use({ viewport: { width: 1280, height: 800 } });

  test('rail presente, nav mobile ausente', async ({ page }) => {
    await loginViaStub(page);
    await page.goto('/');

    // o rail canônico está visível
    await expect(page.locator('aside.tt-sidenav')).toBeVisible();
    // a nav mobile NÃO aparece no desktop (display:none ≥ md)
    await expect(page.getByTestId('mobile-nav')).toBeHidden();

    // byte-compat do <main> ≥768: o padding-bottom volta a ser o respiro normal
    // (var(--pad)), NÃO foi zerado ao sair do inline NEM ganhou o clearance mobile.
    const padBottom = await page.evaluate(() => {
      const main = document.querySelector('main.tt-mainpad');
      return main ? parseFloat(getComputedStyle(main).paddingBottom) : -1;
    });
    expect(padBottom).toBeGreaterThan(0); // não zerado (a regra base .tt-mainpad atua)
    expect(padBottom).toBeLessThan(58); // não é o clearance mobile (só < md)
  });
});
