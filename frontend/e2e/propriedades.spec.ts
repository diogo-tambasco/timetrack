import { test, expect } from '@playwright/test';
import { loginViaStub } from './helpers';

// Fase 25 T-13 (CA-23): propriedades tipadas ponta a ponta — logado (stub) com escrita
// num projeto shared: abre uma task, "Add property" → cria um select "Cliente" com 2
// opções, define o valor numa task; abre a aba Table e vê a coluna "Cliente" com o
// valor; edita inline (troca a opção) → reflete; reload persiste; deleta a def → a
// coluna some da Table. Sem perfil viewer (stub = 1 user — CA-23); o gating viewer é
// teste de componente jsdom (PropertyCell/PropertyRow/CustomProperties specs).
test('propriedades: add property select, valor, Table, edita inline, reload, delete', async ({ page }) => {
  // a remoção da def confirma via window.confirm → aceita todos os diálogos.
  page.on('dialog', (d) => d.accept());

  await loginViaStub(page);

  const sufixo = Date.now();
  const projeto = `Proj Props ${sufixo}`;

  // cria o projeto
  await page.goto('/projetos');
  await page.getByRole('button', { name: /novo projeto/i }).click();
  const proj = page.getByRole('dialog', { name: /novo projeto/i });
  await proj.getByLabel(/nome do projeto/i).fill(projeto);
  await proj.getByRole('button', { name: /criar projeto/i }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);

  // abre o detalhe → aba Tasks
  await page.getByRole('link', { name: projeto }).click();
  await expect(page).toHaveURL(/\/projetos\/[0-9a-f-]+$/);
  await page.getByRole('tab', { name: /tasks/i }).click();

  // cria uma task (New task) → abre o popup central
  await page.getByRole('button', { name: /new task/i }).click();
  const panel = page.getByTestId('center-panel');
  await expect(panel).toBeVisible();

  // Add property → fase 31: o form abre num POPOVER (portal no body, fora do panel).
  // Cria um select "Cliente" com 2 opções: nome → escolhe o tipo "Seleção" na lista
  // de tipos → preenche as 2 opções → Criar.
  await panel.getByLabel('Add property').click();
  const propForm = page.getByRole('dialog', { name: /nova propriedade/i });
  await expect(propForm).toBeVisible();
  await propForm.getByLabel('Nome da nova propriedade').fill('Cliente');
  await propForm.getByRole('option', { name: 'Seleção', exact: true }).click();
  await propForm.getByRole('textbox', { name: 'Opção 2', exact: true }).fill('Arkan');
  // o popover (fase 31) é fixed-positioned abaixo do botão "Add property" (no fim da
  // lista de props) → o botão Criar cai abaixo do viewport e não dá pra clicar.
  // Enter numa opção NÃO-última submete o form (mesma ação do botão Criar).
  await propForm.getByRole('textbox', { name: 'Opção 1', exact: true }).fill('Lauda');
  await propForm.getByRole('textbox', { name: 'Opção 1', exact: true }).press('Enter');

  // a linha custom "Cliente" aparece no popup; define o valor "Lauda"
  const cellNoPopup = panel.getByLabel('Cliente', { exact: true });
  await expect(cellNoPopup).toBeVisible();
  await cellNoPopup.selectOption({ label: 'Lauda' });

  // fecha o popup → aba Table → coluna "Cliente" com o valor
  await page.getByTestId('center-close').click();
  await page.getByRole('tab', { name: /table/i }).click();
  const table = page.getByTestId('task-table');
  await expect(table).toBeVisible();
  await expect(table.getByRole('columnheader', { name: 'Cliente' })).toBeVisible();
  const cellNaTable = table.getByLabel('Cliente', { exact: true });
  await expect(cellNaTable).toHaveValue(/.+/); // tem uma option selecionada (Lauda)

  // edita inline na Table: troca pra "Arkan"
  await cellNaTable.selectOption({ label: 'Arkan' });

  // reload persiste (a Table fica via token table-v2; o valor vem do board)
  await page.reload();
  await page.getByRole('tab', { name: /tasks/i }).click();
  await expect(page.getByTestId('task-table')).toBeVisible();
  const recarregada = page.getByTestId('task-table').getByLabel('Cliente', { exact: true });
  await expect(recarregada).toBeVisible();
  // a option selecionada continua "Arkan" (texto da option escolhida persiste)
  await expect(recarregada.locator('option:checked')).toHaveText('Arkan');

  // deleta a def: reabre a task (clica no NOME, exato — evita os chips de status/prio) → ✕ → confirma
  await page.getByTestId('task-table').getByRole('button', { name: 'New task', exact: true }).click();
  const panel2 = page.getByTestId('center-panel');
  await expect(panel2).toBeVisible();
  await panel2.getByLabel('Remover propriedade Cliente').click();
  await page.getByTestId('center-close').click();

  // a coluna "Cliente" sumiu da Table
  await expect(
    page.getByTestId('task-table').getByRole('columnheader', { name: 'Cliente' }),
  ).toHaveCount(0);
});
