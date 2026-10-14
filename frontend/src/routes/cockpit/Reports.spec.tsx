// Reports.spec — fase 14: a página de Relatórios 2.0 (CA-27..33). Padrão das
// páginas da 02/03/07: mock de ../../lib/api + QueryClientProvider +
// useAuthStore.setState. O rótulo do período SEMPRE deriva do from/to da
// RESPOSTA (CA-08/27). T-08 cobre período/filtros/toggle; T-09 KPIs/donut/
// foco-hora; T-10 objetivos/tabela/vazio/ângulo 4 condicional.
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import Reports from "./Reports";
import { useAuthStore } from "../../lib/auth";
import { browserTZ } from "../../lib/format";
import * as api from "../../lib/api";
import type { PeriodReport } from "../../lib/api";
import { within } from "@testing-library/react";
import { deltaLabel } from "../../components/reports/KpiStrip";

vi.mock("../../lib/api", async (importOriginal) => {
  const mod = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...mod,
    getReport: vi.fn(),
    getPlannedReport: vi.fn(),
    listClients: vi.fn(),
    listProjects: vi.fn(),
    listObjectives: vi.fn(),
  };
});

const ws = "w1";

function relatorio(over: Partial<PeriodReport> = {}): PeriodReport {
  return {
    from: "2026-06-08",
    to: "2026-06-14",
    total_seconds: 18300,
    by_project: [
      {
        project_id: "p1",
        name: "Lauda",
        color: "#b07cd6",
        client_id: "c1",
        client_name: "Lauda.ai",
        total_seconds: 12600,
      },
      {
        project_id: null,
        name: null,
        color: null,
        client_id: null,
        client_name: null,
        total_seconds: 5700,
      },
    ],
    focus: {
      focus_seconds: 15000,
      fragment_seconds: 3300,
      focus_count: 5,
      fragment_count: 4,
      longest_seconds: 5400,
      switches: 7,
    },
    goals: [
      {
        project_id: "p1",
        name: "Lauda",
        color: "#b07cd6",
        goal_seconds: 36000,
        realized_seconds: 12600,
      },
    ],
    objectives: [
      {
        project_id: "p1",
        project_name: "Lauda",
        project_color: "#b07cd6",
        objective_id: "o1",
        objective_name: "Lançar v1",
        status: "open",
        tasks_total: 2,
        tasks_done: 1,
        progress_pct: 50,
        realized_seconds: 5400,
        goal_seconds: 7200,
      },
    ],
    availability: {
      tracked_seconds: 18300,
      expected_seconds: 144000,
      untracked_seconds: 125700,
      tracked_pct: 13,
      expected_basis: "8h/dia × 5 dias úteis",
    },
    focus_by_hour: Array.from({ length: 24 }, (_, h) => ({
      hour: h,
      focus_seconds: h === 16 ? 7200 : 0,
    })),
    kpis: {
      tracked_seconds: 18300,
      deep_focus_pct: 82,
      avg_session_seconds: 3660,
      untracked_seconds: 125700,
    },
    ...over,
  };
}

const vazio = (): PeriodReport =>
  relatorio({
    total_seconds: 0,
    by_project: [],
    objectives: [],
    goals: [],
    focus: {
      focus_seconds: 0,
      fragment_seconds: 0,
      focus_count: 0,
      fragment_count: 0,
      longest_seconds: 0,
      switches: 0,
    },
    availability: {
      tracked_seconds: 0,
      expected_seconds: 0,
      untracked_seconds: 0,
      tracked_pct: 0,
      expected_basis: "",
    },
    focus_by_hour: Array.from({ length: 24 }, (_, h) => ({
      hour: h,
      focus_seconds: 0,
    })),
    kpis: {
      tracked_seconds: 0,
      deep_focus_pct: 0,
      avg_session_seconds: 0,
      untracked_seconds: 0,
    },
  });

function renderPagina() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <Reports />
    </QueryClientProvider>,
  );
}

// fase 22 (CA-18): a ReportTable agora decide table↔cartões por useMediaQuery.
// jsdom não tem matchMedia → sem stub o hook cairia no default mobile (cartões)
// e os testes desktop da tabela quebrariam por AMBIENTE (não comportamento).
// Stubamos matchMedia: matches=desktop. Default = desktop (a tabela densa de
// hoje); o teste mobile abaixo flipa pra cartões. Nenhuma asserção de dado muda.
function stubMatchMedia(desktop: boolean) {
  vi.stubGlobal(
    "matchMedia",
    vi.fn(() => ({
      matches: desktop,
      media: "",
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: () => false,
    })),
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  stubMatchMedia(true); // default = desktop (tabela densa de hoje)
  useAuthStore.setState({
    status: "authenticated",
    me: {
      user: { id: "u1", email: "a@b.c", name: "Ana", avatar_url: null },
      workspaces: [{ id: ws, name: "Workspace de Ana", role: "owner" }],
    },
  });
  vi.mocked(api.getReport).mockResolvedValue(relatorio());
  vi.mocked(api.getPlannedReport).mockResolvedValue({
    week_start: "2026-06-08",
    week_end: "2026-06-14",
    connected: true,
    days: [
      { date: "2026-06-08", planned_seconds: 7200, realized_seconds: 5400 },
    ],
  });
  vi.mocked(api.listClients).mockResolvedValue([
    {
      id: "c1",
      workspace_id: ws,
      name: "Lauda.ai",
      archived_at: null,
      created_at: "2026-01-01T00:00:00Z",
    },
  ]);
  vi.mocked(api.listProjects).mockResolvedValue([
    {
      id: "p1",
      workspace_id: ws,
      client_id: "c1",
      name: "Lauda",
      color: "#b07cd6",
      visibility: "shared",
      weekly_hours_goal: 10,
      archived_at: null,
      owner_user_id: "u1",
    } as never,
  ]);
  vi.mocked(api.listObjectives).mockResolvedValue([]);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("Relatórios 2.0 — período (CA-27)", () => {
  it("default = Semana; busca com period=week, date de hoje e tz do navegador; rótulo do from/to da resposta", async () => {
    renderPagina();
    expect((await screen.findAllByText("Lauda")).length).toBeGreaterThan(0);

    const call = vi.mocked(api.getReport).mock.calls[0];
    expect(call[1].period).toBe("week");
    expect(call[1].tz).toBe(browserTZ());
    expect(call[1].date).toBeTruthy();

    // rótulo SEMPRE do from/to da resposta (08–14/06/2026)
    expect(screen.getByTestId("period-label").textContent).toMatch(
      /08\/06.*14\/06\/2026/,
    );
  });

  it("trocar pro preset Dia refaz a busca com period=day", async () => {
    renderPagina();
    await screen.findAllByText("Lauda");

    vi.mocked(api.getReport).mockResolvedValue(
      relatorio({ from: "2026-06-20", to: "2026-06-20" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Dia" }));

    await waitFor(() => {
      const last = vi.mocked(api.getReport).mock.calls.at(-1)!;
      expect(last[1].period).toBe("day");
    });
    // rótulo de um dia (sem intervalo)
    await waitFor(() =>
      expect(screen.getByTestId("period-label").textContent).toMatch(
        /20\/06\/2026/,
      ),
    );
  });

  it("‹ período anterior navega por passo coerente com o preset (semana − 7d)", async () => {
    renderPagina();
    await screen.findAllByText("Lauda");
    const dateAntes = vi.mocked(api.getReport).mock.calls[0][1].date!;

    fireEvent.click(screen.getByRole("button", { name: /período anterior/i }));
    await waitFor(() => {
      const last = vi.mocked(api.getReport).mock.calls.at(-1)!;
      const d = last[1].date!;
      // 7 dias antes
      const diff =
        (new Date(dateAntes).getTime() - new Date(d).getTime()) / 86400000;
      expect(diff).toBe(7);
    });
  });
});

describe("Relatórios 2.0 — filtros (CA-28)", () => {
  it("escolher um cliente refaz a busca com client_id", async () => {
    renderPagina();
    await screen.findAllByText("Lauda");

    fireEvent.change(screen.getByLabelText("Filtrar por cliente"), {
      target: { value: "c1" },
    });
    await waitFor(() => {
      const last = vi.mocked(api.getReport).mock.calls.at(-1)!;
      expect(last[1].client_id).toBe("c1");
      expect(last[1].tz).toBe(browserTZ());
    });
  });

  it("escolher um projeto refaz a busca com project_id e habilita o select de objetivo", async () => {
    renderPagina();
    await screen.findAllByText("Lauda");

    const objSelect = screen.getByLabelText(
      "Filtrar por objetivo",
    ) as HTMLSelectElement;
    expect(objSelect.disabled).toBe(true); // sem projeto

    fireEvent.change(screen.getByLabelText("Filtrar por projeto"), {
      target: { value: "p1" },
    });
    await waitFor(() => {
      const last = vi.mocked(api.getReport).mock.calls.at(-1)!;
      expect(last[1].project_id).toBe("p1");
    });
    await waitFor(() =>
      expect(
        (screen.getByLabelText("Filtrar por objetivo") as HTMLSelectElement)
          .disabled,
      ).toBe(false),
    );
  });
});

describe("Relatórios 2.0 — toggle Gráficos↔Tabela (CA-28/31)", () => {
  it("alterna pra Tabela e mostra a tt-table; volta pra Gráficos", async () => {
    renderPagina();
    await screen.findAllByText("Lauda");

    expect(document.querySelector("table.tt-table")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Tabela" }));
    expect(await screen.findByTestId("report-table")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Gráficos" }));
    await waitFor(() =>
      expect(document.querySelector("table.tt-table")).toBeNull(),
    );
  });
});

describe("Relatórios 2.0 — KPIs + donut + foco por hora (CA-20/22/24/30)", () => {
  it("a faixa de KPIs renderiza os valores; donut com tracked_pct; 24 barras de foco", async () => {
    renderPagina();
    await screen.findAllByText("Lauda");

    const strip = screen.getByTestId("kpi-strip");
    expect(strip.textContent).toContain("Foco profundo");
    expect(
      within(screen.getByTestId("kpi-deep")).getByText("82%"),
    ).toBeInTheDocument();

    // donut com o % do tracked_pct (13)
    expect(
      within(screen.getByTestId("tracked-donut")).getByText("13%"),
    ).toBeInTheDocument();

    // 24 barras de foco por hora
    const fbh = screen.getByTestId("focus-by-hour");
    expect(fbh.querySelectorAll("[data-testid^='hour-']").length).toBe(24);
  });

  it("sem compare não mostra delta nos KPIs", async () => {
    renderPagina();
    await screen.findAllByText("Lauda");
    expect(screen.queryByTestId("kpi-delta-tracked")).not.toBeInTheDocument();
  });

  it("com compare mostra delta com sinal quando anterior > 0", async () => {
    vi.mocked(api.getReport).mockResolvedValue(
      relatorio({
        previous: {
          total_seconds: 10000,
          kpis: {
            tracked_seconds: 10000,
            deep_focus_pct: 70,
            avg_session_seconds: 3000,
            untracked_seconds: 100000,
          },
        },
      }),
    );
    renderPagina();
    await screen.findAllByText("Lauda");
    fireEvent.click(screen.getByRole("checkbox"));

    await waitFor(() =>
      expect(screen.getByTestId("kpi-delta-tracked")).toBeInTheDocument(),
    );
    // 18300 vs 10000 → +83%
    expect(screen.getByTestId("kpi-delta-tracked").textContent).toBe("+83%");
  });

  it('com compare e anterior = 0 (período novo) mostra "novo" — nunca NaN', async () => {
    vi.mocked(api.getReport).mockResolvedValue(
      relatorio({
        previous: {
          total_seconds: 0,
          kpis: {
            tracked_seconds: 0,
            deep_focus_pct: 0,
            avg_session_seconds: 0,
            untracked_seconds: 0,
          },
        },
      }),
    );
    renderPagina();
    await screen.findAllByText("Lauda");
    fireEvent.click(screen.getByRole("checkbox"));

    await waitFor(() =>
      expect(screen.getByTestId("kpi-delta-tracked").textContent).toBe("novo"),
    );
    expect(document.body.textContent).not.toContain("NaN");
    expect(document.body.textContent).not.toContain("Infinity");
  });
});

describe("deltaLabel (CA-30 — fórmula sem div/0)", () => {
  it("anterior > 0 → sinal; anterior 0 e atual > 0 → novo; ambos 0 → 0%", () => {
    expect(deltaLabel(112, 100)).toBe("+12%");
    expect(deltaLabel(80, 100)).toBe("-20%");
    expect(deltaLabel(50, 0)).toBe("novo");
    expect(deltaLabel(0, 0)).toBe("0%");
    expect(deltaLabel(100, 100)).toBe("+0%");
  });
});

describe("Relatórios 2.0 — objetivos por entrega (CA-16..19)", () => {
  it("objetivo com 1/2 tasks = 50% + realizado; status done marcado", async () => {
    vi.mocked(api.getReport).mockResolvedValue(
      relatorio({
        objectives: [
          {
            project_id: "p1",
            project_name: "Lauda",
            project_color: "#b07cd6",
            objective_id: "o1",
            objective_name: "Lançar v1",
            status: "open",
            tasks_total: 2,
            tasks_done: 1,
            progress_pct: 50,
            realized_seconds: 5400,
            goal_seconds: 7200,
          },
          {
            project_id: "p1",
            project_name: "Lauda",
            project_color: "#b07cd6",
            objective_id: "o2",
            objective_name: "Beta fechado",
            status: "done",
            tasks_total: 3,
            tasks_done: 3,
            progress_pct: 100,
            realized_seconds: 9000,
          },
        ],
      }),
    );
    renderPagina();
    await screen.findByTestId("objectives-section");

    const o1 = screen.getByTestId("objective-o1");
    expect(o1.textContent).toContain("1/2 tarefas (50%)");
    const fill1 = screen.getByTestId("objective-fill-o1") as HTMLElement;
    expect(fill1.style.width).toBe("50%");

    // status done marcado com o selo
    expect(screen.getByTestId("objective-done-o2")).toBeInTheDocument();
  });

  it("seção vazia quando não há objetivo", async () => {
    vi.mocked(api.getReport).mockResolvedValue(relatorio({ objectives: [] }));
    renderPagina();
    await screen.findAllByText("Lauda");
    expect(
      screen.getByText(/nenhum objetivo com atividade ou meta/i),
    ).toBeInTheDocument();
  });
});

describe("Relatórios 2.0 — tabela densa (CA-31)", () => {
  it("uma linha por projeto: projeto, cliente, rastreado, meta, objetivos concluídos/total — SEM colunas fabricadas", async () => {
    renderPagina();
    await screen.findAllByText("Lauda");
    fireEvent.click(screen.getByRole("button", { name: "Tabela" }));

    const table = await screen.findByTestId("report-table");
    // cabeçalhos: só as 5 colunas do contrato
    const heads = Array.from(table.querySelectorAll("thead th")).map(
      (th) => th.textContent,
    );
    expect(heads).toEqual([
      "Projeto",
      "Cliente",
      "Rastreado",
      "Meta",
      "Objetivos",
    ]);
    // NÃO fabricar colunas de workspace por projeto
    expect(table.textContent).not.toMatch(/não-rastreado/i);
    expect(table.textContent).not.toMatch(/sessões/i);

    // linha do projeto p1: rastreado (3h30) + objetivos 0/1 (1 objetivo open)
    const row = screen.getByTestId("row-p1");
    expect(row.textContent).toContain("Lauda.ai"); // cliente
    expect(row.textContent).toContain("3h30"); // 12600s
    expect(row.textContent).toContain("0/1"); // 1 objetivo, 0 concluído
  });

  // fase 22 T-05 (CA-08): < md a tabela vira CARTÕES (label:valor, TODOS os
  // dados; nada removido). Estrutura (jsdom) — overflow a 375px = T-10/CA-19.
  it("< md: a tabela vira cartões empilhados com os mesmos dados (sem <table>)", async () => {
    stubMatchMedia(false); // mobile
    renderPagina();
    await screen.findAllByText("Lauda");
    fireEvent.click(screen.getByRole("button", { name: "Tabela" }));

    const cards = await screen.findByTestId("report-table");
    // NÃO é mais uma <table> no mobile
    expect(cards.querySelector("table")).toBeNull();
    expect(document.querySelector("table.tt-table")).toBeNull();

    // mesmos dados que a tabela desktop, em label:valor
    const row = screen.getByTestId("row-p1");
    expect(row.textContent).toContain("Lauda"); // projeto
    expect(row.textContent).toContain("Cliente");
    expect(row.textContent).toContain("Lauda.ai");
    expect(row.textContent).toContain("Rastreado");
    expect(row.textContent).toContain("3h30");
    expect(row.textContent).toContain("Objetivos");
    expect(row.textContent).toContain("0/1");
  });
});

describe("Relatórios 2.0 — ângulo 4 condicional + vazio (CA-32/33)", () => {
  it("Planejado×real aparece só no preset Semana (oculto no Dia)", async () => {
    renderPagina();
    await screen.findAllByText("Lauda");
    // na Semana: a seção existe
    expect(screen.getByText("Planejado × real")).toBeInTheDocument();

    // troca pro Dia → some
    fireEvent.click(screen.getByRole("button", { name: "Dia" }));
    await waitFor(() =>
      expect(screen.queryByText("Planejado × real")).not.toBeInTheDocument(),
    );
  });

  it("período sem dados → estados vazios sem NaN", async () => {
    vi.mocked(api.getReport).mockResolvedValue(vazio());
    renderPagina();

    expect(
      await screen.findByText("Nada cronometrado nesta semana."),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/nenhum objetivo com atividade ou meta/i),
    ).toBeInTheDocument();
    expect(document.body.textContent).not.toContain("NaN");
    // donut 0%
    expect(
      within(screen.getByTestId("tracked-donut")).getByText("0%"),
    ).toBeInTheDocument();
  });
});
