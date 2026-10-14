// reminders.spec — fase 20 T-10 (CA-30): o fluxo de lembretes ponta a ponta.
// (1) um reminder com remind_at no PASSADO + o worker tickando (REMINDER_TICK=1s
//     no playwright.config) + o app aberto → o banner de LEMBRETE aparece
//     (notice pela cadeia WS da fase 04, igual ao corte da fase 16).
// (2) na lista "Meus lembretes" (/ajustes), um pendente pode ser CANCELADO e
//     some da lista.
// O Web Push real NÃO é exercitado headless (tem unit no backend). O googlestub
// só autentica diogo.fixture@example.com — o carve-out viewer-assignee é teste
// de API Go (mesma limitação das fases 18/19).
//
// Red honesto: rodado antes do wiring de UI/worker → o banner não aparece e o
// grupo "Meus lembretes" não existe (exceção mecânica: nenhuma — é red estrutural
// genuíno). Liberar a :8080 de qualquer dev server alheio antes do gate.
import { test, expect } from '@playwright/test';
import { loginViaStub, seedReminder } from './helpers';

test('CA-30: lembrete vencido mostra o banner; cancelar na lista remove o pendente', async ({
  page,
}) => {
  await loginViaStub(page);
  await expect(page.getByTestId('timerbar')).toBeVisible();

  // (1) banner do lembrete vencido — chega pelo WS quando o worker despacha.
  const lembrada = `Pagar boleto ${Date.now()}`;
  seedReminder(lembrada, 'past');

  const banner = page.getByTestId('notice-banner');
  await expect(banner).toBeVisible({ timeout: 15_000 });
  await expect(banner).toContainText(/Lembrete:/i);
  await expect(banner).toContainText(lembrada);

  // dispensa o banner (sem efeito no servidor)
  await page.getByRole('button', { name: /dispensar aviso/i }).click();
  await expect(banner).toBeHidden();

  // (2) lista "Meus lembretes" em /ajustes: um PENDENTE futuro aparece e some ao cancelar.
  const pendente = `Revisar contrato ${Date.now()}`;
  seedReminder(pendente, 'future');

  await page.goto('/ajustes');
  const grupo = page.getByTestId('reminders-group');
  await expect(grupo).toBeVisible();

  const item = grupo.getByTestId('reminder-item').filter({ hasText: pendente });
  await expect(item).toBeVisible({ timeout: 10_000 });

  // cancelar → o item some da lista (a query invalida no onSuccess)
  await item.getByRole('button', { name: /cancelar lembrete/i }).click();
  await expect(grupo.getByTestId('reminder-item').filter({ hasText: pendente })).toHaveCount(0, {
    timeout: 10_000,
  });
});
