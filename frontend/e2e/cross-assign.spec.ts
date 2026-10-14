import { test, expect } from '@playwright/test';
import { loginViaStub, seedWorkspaceMember, waitTaskPatch } from './helpers';

// Fase 24 T-16 (reconcilia o cross-assign.spec da fase 18 — CA-28.1): Responsável/
// Solicitante MIGRARAM pro popup central (TaskPanel), via quick-add. LIMITAÇÃO DO
// STUB — o googlestub autentica SÓ o owner; cobre o que o owner faz headless:
//   1. atribuir uma task a um colega B (SEEDED) no popup → PERSISTE (reabrir mostra B);
//   2. atribuir uma task a SI + pedida por si → aparece em /minhas-tarefas (2 abas).
// O carve-out viewer-assignee + a visão de B são cobertos por Go + jsdom.
test('cross-assign: atribuir a colega persiste; auto-atribuição aparece em Minhas tasks', async ({ page }) => {
  await loginViaStub(page);

  const colega = `Beto ${Date.now()}`;
  seedWorkspaceMember({
    googleId: `g-colega-${Date.now()}`,
    name: colega,
    email: `colega.${Date.now()}@example.com`,
    role: 'member',
  });

  const sufixo = Date.now();
  const projeto = `Proj CrossAssign ${sufixo}`;
  const taskColega = `Pra o colega ${sufixo}`;
  const taskMinha = `Minha task ${sufixo}`;

  // cria o projeto e abre a aba Tasks
  await page.goto('/projetos');
  await page.getByRole('button', { name: /novo projeto/i }).click();
  const proj = page.getByRole('dialog', { name: /novo projeto/i });
  await proj.getByLabel(/nome do projeto/i).fill(projeto);
  await proj.getByRole('button', { name: /criar projeto/i }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await page.getByRole('link', { name: projeto }).click();
  await expect(page).toHaveURL(/\/projetos\/[0-9a-f-]+$/);
  // fase 28 (CA-19): o shell abre na Overview; vai pra Tasks
  await page.getByRole('tab', { name: /tasks/i }).click();

  // (1) task atribuída ao colega B pelo popup. Serializa os PATCHes (nome → assignee)
  // esperando cada resposta: dois PATCHes em voo na mesma task fazem o board assentar
  // em estado parcial (CA-20). Espelha o uso real (muda o responsável depois do nome).
  await page.getByRole('button', { name: /add em to do/i }).click();
  let panel = page.getByTestId('center-panel');
  await panel.getByLabel('Título da task').fill(taskColega);
  let patched = waitTaskPatch(page);
  await panel.getByLabel('Título da task').press('Enter'); // salva o nome (CA-21)
  await patched;
  // o select de membros precisa ter carregado o colega seeded
  await expect(panel.getByLabel('Responsável').getByRole('option', { name: colega })).toHaveCount(1);
  patched = waitTaskPatch(page);
  await panel.getByLabel('Responsável').selectOption({ label: colega });
  await patched;
  await page.getByTestId('center-close').click();
  await expect(page.getByTestId('center-panel')).toHaveCount(0);

  // PERSISTE: os PATCHes (nome+responsável) já voltaram 200 (waitTaskPatch) → estão
  // no banco. Recarrega pra ler do backend (fonte da verdade) — determinístico, sem
  // depender da convergência do refetch ao vivo do board, que sob carga paralela
  // pesada do E2E (5 specs concorrentes martelando 1 backend) às vezes não assenta no
  // tempo do teste (o reflexo ao vivo / CA-20 é coberto isolado + jsdom). Abre a task
  // na Table e confirma o responsável = B.
  await page.reload();
  // fase 28 (CA-19): o reload reabre na Overview; volta pra Tasks antes da Table.
  await page.getByRole('tab', { name: /tasks/i }).click();
  await page.getByRole('tab', { name: /table/i }).click();
  await page.getByTestId('task-table').getByRole('button', { name: taskColega, exact: true }).click();
  panel = page.getByTestId('center-panel');
  // o select "Responsável" é populado pelo board APÓS o reload (async); poll até
  // assentar no colega. Ler de forma síncrona (selectedOptions[0] num único tick)
  // pegava o default "— sem responsável" ANTES do board chegar — race de LEITURA
  // (a persistência está correta: o snapshot mostra o colega [selected]). Conserto
  // de estabilidade do gate: determinístico em db-test limpo OU acumulado.
  await expect
    .poll(async () =>
      panel
        .getByLabel('Responsável')
        .evaluate((el) => (el as HTMLSelectElement).selectedOptions[0]?.textContent ?? ''),
    )
    .toContain(colega);
  await page.getByTestId('center-close').click();

  // (2) task atribuída a SI + pedida por si
  await page.getByRole('tab', { name: /board/i }).click();
  await page.getByRole('button', { name: /add em to do/i }).click();
  panel = page.getByTestId('center-panel');
  // serializa nome → responsável → solicitante (cada PATCH assentado antes do
  // próximo) pra a auto-atribuição persistir inteira ANTES do page.goto (um reload
  // com PATCH em voo cancelaria a escrita → flake — CA-28.1).
  await panel.getByLabel('Título da task').fill(taskMinha);
  let p = waitTaskPatch(page);
  await panel.getByLabel('Título da task').press('Enter'); // salva o nome
  await p;
  p = waitTaskPatch(page);
  await panel.getByLabel('Responsável').selectOption({ label: 'Diogo Fixture' });
  await p;
  p = waitTaskPatch(page);
  await panel.getByLabel('Solicitante').selectOption({ label: 'Diogo Fixture' });
  await p;
  await page.getByTestId('center-close').click();
  await expect(page.getByTestId('center-panel')).toHaveCount(0);

  // /minhas-tarefas aba "Minhas tasks" mostra a auto-atribuída
  await page.goto('/minhas-tarefas');
  await expect(page.getByText(taskMinha)).toBeVisible();

  // aba "Pedidas por mim" mostra a mesma
  await page.getByRole('tab', { name: /pedidas por mim/i }).click();
  await expect(page.getByText(taskMinha)).toBeVisible();
});
