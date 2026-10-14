// T-12 da fase 14 (CA-34) — fluxo de Relatórios 2.0. Arquivo PARALELO: o
// relatório não muta o timer (plan §6). Toda asserção numérica é escopada ao
// PROJETO ÚNICO do teste (nome com Date.now()) — outros specs rodam em paralelo
// criando entries do MESMO usuário, então números globais só têm asserção de
// presença/render. Objetivo+tasks semeados via API (POST objectives/tasks);
// entries via POST /entries.
//
// Exceção ao red estrutural documentada (precedente do relatorios.spec da 07):
// o app é entregue por T-01..T-11; red honesto = spec rodado ANTES dos ajustes.
import { test, expect } from "@playwright/test";
import { loginViaStub } from "./helpers";

test("CA-34: período/filtros/objetivos/compare/tabela num projeto único", async ({
  page,
}) => {
  await loginViaStub(page);

  const sufixo = Date.now();
  const cliente = `Cliente R2 ${sufixo}`;
  const projeto = `Projeto R2 ${sufixo}`;

  const me = (await (await page.request.get("/api/v1/me")).json()) as {
    workspaces: { id: string }[];
  };
  const wsId = me.workspaces[0].id;
  const base = `/api/v1/workspaces/${wsId}`;

  // cliente + projeto único com meta de 1h/semana
  const clienteId = (
    (await (
      await page.request.post(`${base}/clients`, { data: { name: cliente } })
    ).json()) as {
      id: string;
    }
  ).id;
  const projResp = await page.request.post(`${base}/projects`, {
    data: {
      name: projeto,
      color: "#4f9dde",
      visibility: "shared",
      client_id: clienteId,
      weekly_hours_goal: 1,
    },
  });
  expect(projResp.status()).toBe(201);
  const projId = ((await projResp.json()) as { id: string }).id;

  // objetivo com meta + 2 tasks (1 done)
  const objResp = await page.request.post(
    `${base}/projects/${projId}/objectives`,
    {
      data: { name: `Objetivo ${sufixo}`, goal_hours: 2 },
    },
  );
  expect(objResp.status()).toBe(201);
  const objId = ((await objResp.json()) as { id: string }).id;

  const tk1 = (
    (await (
      await page.request.post(`${base}/tasks`, {
        data: { project_id: projId, name: `t1 ${sufixo}`, objective_id: objId },
      })
    ).json()) as { id: string }
  ).id;
  await page.request.post(`${base}/tasks`, {
    data: { project_id: projId, name: `t2 ${sufixo}`, objective_id: objId },
  });
  // conclui a 1ª task → 1/2 done (50%)
  const patchTk = await page.request.patch(`${base}/tasks/${tk1}`, {
    data: { status: "done" },
  });
  expect(patchTk.status()).toBe(200);

  // 2 entries fechadas de HOJE: 40min ligada à task (foco) + 10min (fragmento)
  const hoje = new Date();
  const at = (h: number, m: number) =>
    new Date(
      hoje.getFullYear(),
      hoje.getMonth(),
      hoje.getDate(),
      h,
      m,
      0,
    ).toISOString();

  const e1 = await page.request.post(`${base}/entries`, {
    data: {
      description: `r2 ${sufixo}`,
      project_id: projId,
      task_id: tk1,
      started_at: at(8, 0),
      stopped_at: at(8, 40),
    },
  });
  expect(e1.status()).toBe(201);
  const e2 = await page.request.post(`${base}/entries`, {
    data: {
      description: `r2 ${sufixo}`,
      project_id: projId,
      started_at: at(9, 0),
      stopped_at: at(9, 10),
    },
  });
  expect(e2.status()).toBe(201);

  await page.goto("/relatorios");

  // preset Semana (default): ângulo 1 mostra o projeto com 0h50
  const barra = page.getByTestId(`bar-${projId}`);
  await expect(barra).toBeVisible();
  await expect(barra).toContainText(projeto);
  await expect(barra).toContainText("0h50");

  // seção objetivos: 1/2 tarefas (50%) + realizado (0h40 ligado à task)
  const objSection = page.getByTestId(`objective-${objId}`);
  await expect(objSection).toBeVisible();
  await expect(objSection).toContainText("1/2 tarefas (50%)");
  await expect(page.getByTestId(`objective-fill-${objId}`)).toHaveAttribute(
    "style",
    /width:\s*50%/,
  );
  await expect(objSection).toContainText("0h40"); // realizado no período (entry da task)

  // filtra por aquele projeto → os números se mantêm escopados (0h50)
  await page.getByLabel("Filtrar por projeto").selectOption(projId);
  await expect(page.getByTestId(`bar-${projId}`)).toContainText("0h50");
  // com o projeto filtrado, a barra do projeto segue sendo a única (100%)
  await expect(page.getByTestId(`bar-fill-${projId}`)).toHaveAttribute(
    "style",
    /width:\s*100%/,
  );

  // troca pro preset Mês → o rótulo do período muda (intervalo do mês)
  const labelSemana = await page.getByTestId("period-label").textContent();
  await page.getByRole("button", { name: "Mês" }).click();
  await expect(page.getByTestId("period-label")).not.toHaveText(
    labelSemana ?? "",
  );
  // o projeto segue aparecendo no mês
  await expect(page.getByTestId(`bar-${projId}`)).toBeVisible();

  // liga comparar → o período anterior do projeto novo é vazio (anterior=0) →
  // o KPI de rastreado mostra "novo" (CA-30/34), determinístico e sem NaN
  await page.getByRole("checkbox", { name: /comparar/i }).check();
  await expect(page.getByTestId("kpi-delta-tracked")).toHaveText("novo");
  await expect(page.locator("body")).not.toContainText("NaN");

  // abre a Tabela → linha do projeto com rastreado (0h50) e objetivos 0/1
  await page.getByRole("button", { name: "Tabela" }).click();
  const row = page.getByTestId(`row-${projId}`);
  await expect(row).toBeVisible();
  await expect(row).toContainText("0h50");
  await expect(row).toContainText("0/1"); // 1 objetivo (open), 0 concluído
});
