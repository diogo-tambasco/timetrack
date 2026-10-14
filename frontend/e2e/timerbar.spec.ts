import { test, expect } from '@playwright/test';
import { loginViaStub } from './helpers';

// CA-09.1 (fase 00, atualizado pela fase 03): a barra de timer global segue
// SEMPRE visível no topo, em todas as rotas, com dígitos mono. O placeholder
// desabilitado da fase 00 era explicitamente provisório ("timer real é fase
// 03") — o comportamento funcional da barra (start/stop/pause/reload) é
// coberto por cronometro.spec.ts e pelos specs Vitest do TimerBar.
// As asserções aqui são agnósticas ao ESTADO do timer: os workers do
// Playwright compartilham o único usuário do googlestub, então outro spec
// pode estar com timer rodando (ou em pausa) enquanto este navega.
test('TimerBar global visível em todas as rotas (CA-09.1)', async ({ page }) => {
  await loginViaStub(page);

  const rotas = ['/hoje', '/calendario', '/projetos', '/clientes', '/relatorios', '/workspace', '/foco', '/ajustes'];

  for (const rota of rotas) {
    await page.goto(rota);

    const bar = page.getByTestId('timerbar');
    await expect(bar, `barra ausente em ${rota}`).toBeVisible();

    // dígitos mono presentes em qualquer estado: 0:00:00 ociosa, h:mm:ss
    // rodando, mm:ss na pausa
    const digits = bar.locator('.mono');
    await expect(digits, `dígitos ausentes em ${rota}`).toHaveText(/^\d+:\d{2}(:\d{2})?$/);
  }
});
