// T-07 da fase 07 (CA-22) — fluxo nº5 do TDD.md §4, recorte 3 ângulos.
// Arquivo PARALELO: o relatório não muta o timer (plan §6). Toda asserção
// numérica é escopada ao PROJETO ÚNICO do teste (nome com Date.now()) —
// o cronometro.spec roda em paralelo criando entries do MESMO usuário, e
// números globais (focus/total) só têm asserção de presença/render.
//
// Exceção ao red estrutural documentada (precedente do T-19 da 03): o app
// exercitado é entregue por T-01..T-06; red honesto = spec rodado ANTES de
// qualquer ajuste de app.
import { test, expect } from '@playwright/test';
import { loginViaStub } from './helpers';

test('CA-22: projeto com meta + 2 entries → ângulos 1 e 3 com números certos, foco renderizado', async ({
  page,
}) => {
  await loginViaStub(page);

  const sufixo = Date.now();
  const cliente = `Cliente Relatório ${sufixo}`;
  const projeto = `Projeto Relatório ${sufixo}`;

  const me = (await (await page.request.get('/api/v1/me')).json()) as {
    workspaces: { id: string }[];
  };
  const wsId = me.workspaces[0].id;

  // cliente + projeto único com meta de 1h/semana, via API
  const clienteResp = await page.request.post(`/api/v1/workspaces/${wsId}/clients`, {
    data: { name: cliente },
  });
  expect(clienteResp.status()).toBe(201);
  const clienteId = ((await clienteResp.json()) as { id: string }).id;

  const projResp = await page.request.post(`/api/v1/workspaces/${wsId}/projects`, {
    data: {
      name: projeto,
      color: '#4f9dde',
      visibility: 'shared',
      client_id: clienteId,
      weekly_hours_goal: 1,
    },
  });
  expect(projResp.status()).toBe(201);
  const projId = ((await projResp.json()) as { id: string }).id;

  // 2 entries fechadas de HOJE via POST /entries: 40min (foco) + 10min
  // (fragmento) = 0h50 — semeadas de madrugada pra nunca colidir com o dia
  // seguinte no fuso local
  const hoje = new Date();
  const at = (h: number, m: number) => {
    const d = new Date(hoje.getFullYear(), hoje.getMonth(), hoje.getDate(), h, m, 0);
    return d.toISOString();
  };
  for (const [start, stop] of [
    [at(8, 0), at(8, 40)],
    [at(9, 0), at(9, 10)],
  ]) {
    const r = await page.request.post(`/api/v1/workspaces/${wsId}/entries`, {
      data: {
        description: `relatorio ${sufixo}`,
        project_id: projId,
        started_at: start,
        stopped_at: stop,
      },
    });
    expect(r.status()).toBe(201);
  }

  await page.goto('/relatorios');

  // ângulo 1: o projeto único aparece com 0h50 e o cliente resolvido
  const barra = page.getByTestId(`bar-${projId}`);
  await expect(barra).toBeVisible();
  await expect(barra).toContainText(projeto);
  await expect(barra).toContainText(cliente);
  await expect(barra).toContainText('0h50');

  // ângulo 3: GoalBar do projeto com realizado 0h50 de 1h00
  const metas = page.getByTestId('goals-section');
  const linhaMeta = metas.locator('div', { hasText: projeto }).first();
  await expect(linhaMeta).toContainText('0h50');
  await expect(linhaMeta).toContainText('1h00');

  // ângulo 2: seção presente com números renderizados (sem igualdade
  // global — outros testes podem ter semeado entries hoje)
  const focus = page.getByTestId('focus-stats');
  await expect(focus).toBeVisible();
  await expect(focus).toContainText('Maior bloco');
  await expect(focus.locator('.mono').first()).toHaveText(/\d+h\d{2}/);

  // rótulo do período veio da resposta do servidor (fase 14: o WeekNav foi
  // aposentado pelo PeriodNav — D-10; testid week-label → period-label; o
  // formato/dado da semana é idêntico, só o seletor mudou pela reescrita da página)
  await expect(page.getByTestId('period-label')).toHaveText(/\d{2}\/\d{2} – \d{2}\/\d{2}\/\d{4}/);
});
