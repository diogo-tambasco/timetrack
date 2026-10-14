// dashboard.spec — fase 13 T-09 (CA-25, parte PARALELA): o coração da fase é
// o P3 ("virou o dia, sumiram minhas tasks"). Arquivo PARALELO: NÃO muta o
// timer global (só semeia entries fechadas via API + navega) — as partes que
// dependem de timer rodando vivem no arquivo SERIAL cronometro.spec.ts.
// Asserções escopadas a um sufixo único (db-test reusado entre runs).
import { test, expect } from '@playwright/test';
import { loginViaStub } from './helpers';

test('CA-25 (P3): entrada de ONTEM aparece em "Recentes"; atalho Foco e nav Hoje funcionam', async ({
  page,
}) => {
  await loginViaStub(page);

  const sufixo = Date.now();
  const projeto = `Projeto Dash ${sufixo}`;
  const descricao = `Trampo de ontem ${sufixo}`;

  const me = (await (await page.request.get('/api/v1/me')).json()) as {
    workspaces: { id: string }[];
  };
  const wsId = me.workspaces[0].id;

  // projeto único (pro chip cor+nome na linha — CA-07)
  const projResp = await page.request.post(`/api/v1/workspaces/${wsId}/projects`, {
    data: { name: projeto, color: '#4f9dde', visibility: 'shared' },
  });
  expect(projResp.status()).toBe(201);
  const projId = ((await projResp.json()) as { id: string }).id;

  // entry FECHADA de ONTEM (10:00–10:30 local) via API — prova P3: dado de dia
  // anterior visível na home, não some na virada do dia
  const ontem = new Date();
  ontem.setDate(ontem.getDate() - 1);
  const at = (h: number, m: number) =>
    new Date(ontem.getFullYear(), ontem.getMonth(), ontem.getDate(), h, m, 0).toISOString();
  const seedResp = await page.request.post(`/api/v1/workspaces/${wsId}/entries`, {
    data: { description: descricao, project_id: projId, started_at: at(10, 0), stopped_at: at(10, 30) },
  });
  expect(seedResp.status()).toBe(201);

  // abre a home (Dashboard)
  await page.goto('/');
  await expect(page.getByRole('heading', { name: /bom trabalho/i })).toBeVisible();

  // a entrada de ontem aparece em "Recentes" sob o cabeçalho "Ontem" (P3)
  const linha = page.locator('[data-testid^="recent-entry-"]', { hasText: descricao });
  await expect(linha).toBeVisible();
  await expect(linha).toContainText(projeto);
  await expect(page.getByText('Ontem', { exact: true })).toBeVisible();

  // atalho Modo Foco → /foco
  await page.getByRole('link', { name: /modo foco/i }).click();
  await expect(page).toHaveURL(/\/foco$/);

  // nav "Hoje" da SideNav volta pra home (Dashboard) — a partir de uma página
  // normal (o overlay do Foco cobriria o rail)
  await page.goto('/projetos');
  await page.getByRole('link', { name: 'Hoje' }).click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole('heading', { name: /bom trabalho/i })).toBeVisible();
});
