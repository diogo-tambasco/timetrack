// calendario.spec — fase 11 T-11 (CA-27), Calendário 2.0. Google stubbado: o
// connect passa pela dança OAuth do googlestub (concede o scope de calendar) e
// os eventos vêm do calendarstub (:9101, ancorados no timeMin → sempre na
// janela pedida). Reescreve o E2E da 06 (lista → grade): a fidelidade dos
// dados segue assertada, muda a forma (desvio D-09).
//
// Exceção ao red estrutural documentada (precedente do T-19 da 03 / T-08 da
// 06): o app é entregue por T-01..T-10; red honesto = spec rodado ANTES do
// wiring da página.
import { test, expect } from '@playwright/test';
import { loginViaStub, resetCalendarConnection, resetCalendarSettings } from './helpers';

test('CA-27: grade da semana → troca Dia/Mês → popover → view padrão persiste', async ({ page }) => {
  await loginViaStub(page);
  // db-test persistente + user único do stub: zera a conexão pra começar
  // SEMPRE desconectado (senão a 1ª conexão gruda e o CTA some na 2ª rodada)
  // E zera as prefs (senão o 'day' salvo no fim deste teste faz o próximo run
  // abrir no Dia e quebrar a asserção da semana).
  resetCalendarConnection();
  resetCalendarSettings();

  // começa desconectado → CTA (CA-26)
  await page.goto('/calendario');
  const connect = page.getByTestId('connect-calendar');
  await expect(connect).toBeVisible();

  // conecta: dança OAuth stubbada (scope calendar) → volta pra /calendario
  await connect.click();
  await expect(page).toHaveURL(/\/calendario$/);

  // conectado → grade da SEMANA (default_view) com o evento posicionado (CA-05/09)
  await expect(page.getByTestId('time-grid')).toBeVisible();
  await expect(page.getByTestId('day-column')).toHaveCount(7);
  await expect(page.getByText('Reunião do dia')).toBeVisible();

  // clique no evento → popover com "Abrir no Google" (CA-21)
  await page.getByTestId('cal-event').first().click();
  await expect(page.getByTestId('event-popover')).toBeVisible();
  const link = page.getByTestId('open-in-google');
  await expect(link).toHaveAttribute('href', /calendar\.google\.com/);
  await page.keyboard.press('Escape');
  await expect(page.getByTestId('event-popover')).toHaveCount(0);

  // troca pra Mês → grade do mês com o mesmo evento (CA-07/18)
  await page.getByRole('button', { name: 'Mês', exact: true }).click();
  await expect(page.getByTestId('month-grid')).toBeVisible();
  await expect(page.getByText('Reunião do dia')).toBeVisible();

  // troca pra Dia → 1 coluna (CA-06/20)
  await page.getByRole('button', { name: 'Dia', exact: true }).click();
  await expect(page.getByTestId('time-grid')).toBeVisible();
  await expect(page.getByTestId('day-column')).toHaveCount(1);

  // muda a view padrão nos Ajustes pra Dia e salva (CA-25). Escopado ao card
  // do Calendário (o /ajustes tem 2 grupos com botão "Salvar").
  await page.goto('/ajustes');
  const calCard = page.locator('section.tt-card', { hasText: 'View padrão' });
  await calCard.getByRole('button', { name: 'Dia', exact: true }).click();
  await calCard.getByRole('button', { name: /salvar/i }).click();
  await expect(calCard.getByText(/Salvo/i)).toBeVisible();

  // reload /calendario → abre na NOVA view padrão (persistência backend, CA-23)
  await page.goto('/calendario');
  await expect(page.getByTestId('time-grid')).toBeVisible();
  await expect(page.getByTestId('day-column')).toHaveCount(1);
});
