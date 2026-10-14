import { test, expect } from '@playwright/test';
import { loginViaStub, waitTaskPatch } from './helpers';

// Fase 24 T-16 (reconcilia o tarefas.spec da fase 12 — CA-28.1): a fase 24 é dona
// do board e SUBSTITUIU a UI da 12 (3 colunas + TaskModal + ViewToggle binário)
// pelo board de 4 colunas DnD + popup central (TaskPanel) + segmented
// Board/List/Table. Aqui o ciclo da task no NOVO fluxo: quick-add → popup → muda o
// status pelo chip → reload persiste. A conclusão automática do objetivo (CA-05)
// é provada em jsdom (autoClose.spec — a Overview reflete done%) já que a aba
// Tasks da 24 não cria objetivos (Goals = fase 29). Sem perfil viewer (stub = 1
// user); o gating é jsdom.
test('aba Tasks: quick-add por coluna, popup, muda status pelo chip e persiste', async ({ page }) => {
  await loginViaStub(page);

  const sufixo = Date.now();
  const projeto = `Proj Tarefas ${sufixo}`;
  const task = `Task do board ${sufixo}`;

  // cria o projeto
  await page.goto('/projetos');
  await page.getByRole('button', { name: /novo projeto/i }).click();
  const proj = page.getByRole('dialog', { name: /novo projeto/i });
  await proj.getByLabel(/nome do projeto/i).fill(projeto);
  await proj.getByRole('button', { name: /criar projeto/i }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);

  // abre o detalhe → tab-shell abre na Overview (fase 28, CA-19); vai pra Tasks
  await page.getByRole('link', { name: projeto }).click();
  await expect(page).toHaveURL(/\/projetos\/[0-9a-f-]+$/);
  await page.getByRole('tab', { name: /tasks/i }).click();
  await expect(page.getByTestId('board-col-todo')).toBeVisible();

  // quick-add na coluna To do → popup; renomeia. Serializa os PATCHes (nome →
  // status) esperando cada resposta: dois PATCHes em voo na mesma task fazem o board
  // assentar em estado parcial (CA-20). Espelha o uso real (muda o status depois do
  // nome assentar).
  await page.getByRole('button', { name: /add em to do/i }).click();
  const panel = page.getByTestId('center-panel');
  const title = panel.getByLabel('Título da task');
  await title.fill(task);
  let patched = waitTaskPatch(page);
  await title.blur();
  await patched;

  // muda o status pra In progress (todo→doing). Fase 31: o chip vira um DROPDOWN —
  // clicar abre o listbox (portal), escolher "In progress" dispara o PATCH.
  await panel.getByLabel('Status').click(); // abre o dropdown
  patched = waitTaskPatch(page);
  await page.getByRole('option', { name: 'In progress' }).click();
  await patched;
  await page.getByTestId('center-close').click();
  await expect(page.getByTestId('center-panel')).toHaveCount(0);

  // PERSISTE: os PATCHes (nome+status) já voltaram 200 (waitTaskPatch) → estão no
  // banco. Recarrega pra ler do backend (fonte da verdade) e confirma que a task
  // está em In progress (doing). Determinístico, sem depender da convergência do
  // refetch ao vivo do board sob carga paralela pesada do E2E (o reflexo ao vivo /
  // CA-20 é coberto isolado + jsdom).
  await page.reload();
  // fase 28 (CA-19): o reload reabre na Overview; volta pra Tasks pra ver o board
  await page.getByRole('tab', { name: /tasks/i }).click();
  await expect(page.getByTestId('board-col-doing').getByText(task)).toBeVisible();
});
