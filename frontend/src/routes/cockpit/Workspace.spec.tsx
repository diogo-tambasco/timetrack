// Workspace.spec — T-02/T-03 da fase 10 (CA-03..09). Padrão das páginas:
// mock de ../../lib/api + QueryClientProvider + useAuthStore.setState.
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import Workspace from "./Workspace";
import { useAuthStore, useCurrentWorkspace } from "../../lib/auth";
import * as api from "../../lib/api";
import type { WorkspaceDetail, WorkspaceRole } from "../../lib/api";

vi.mock("../../lib/api", async (importOriginal) => {
  const mod = await importOriginal<typeof import("../../lib/api")>();
  return { ...mod, getWorkspace: vi.fn() };
});

const detalhe = (id: string, name: string): WorkspaceDetail => ({
  id,
  name,
  role: "owner",
  created_at: "2026-06-01T10:00:00Z",
  members: [
    { user_id: "u1", name: "Ana", email: "ana@tt.dev", role: "owner" },
    { user_id: "u2", name: "Carlos", email: "carlos@tt.dev", role: "member" },
    { user_id: "u3", name: "Vera", email: "vera@tt.dev", role: "viewer" },
  ],
});

function setMe(
  workspaces: { id: string; name: string; role: WorkspaceRole }[],
) {
  useAuthStore.setState({
    status: "authenticated",
    me: {
      user: { id: "u1", email: "ana@tt.dev", name: "Ana", avatar_url: null },
      workspaces,
    },
    activeWorkspaceId: null,
  });
}

// sonda do CA-06: imprime o id que o RESTO do app enxerga como ativo
function ProbeActive() {
  const ws = useCurrentWorkspace();
  return <div data-testid="probe-active">{ws?.id}</div>;
}

let qc: QueryClient;

function renderPagina() {
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <Workspace />
      <ProbeActive />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  setMe([{ id: "w1", name: "Workspace de Ana", role: "owner" }]);
  vi.mocked(api.getWorkspace).mockImplementation(async (id) =>
    detalhe(id, id === "w1" ? "Workspace de Ana" : "Time Primor"),
  );
});

describe("página Workspace — leitura (T-02)", () => {
  // CA-03 — nome do workspace ativo + membros com nome/email
  it("mostra o nome do workspace e a lista de membros", async () => {
    renderPagina();

    // espera os MEMBROS (a query): o nome tem fallback síncrono do /me
    expect(await screen.findByText("Carlos")).toBeInTheDocument();
    expect(screen.getByTestId("workspace-name")).toHaveTextContent(
      "Workspace de Ana",
    );
    expect(screen.getByText("carlos@tt.dev")).toBeInTheDocument();
    expect(screen.getByText("Vera")).toBeInTheDocument();
  });

  // CA-04 — badges de role PT-BR; dono em destaque (.tt-role--owner).
  // A leitura por badges é a vista de member/viewer (CA-29/34): o owner
  // troca o badge pelos controles de gestão (testado no spec da fase 17).
  it("roles viram badges Dono/Membro/Viewer com o dono em destaque", async () => {
    setMe([{ id: "w1", name: "Workspace de Ana", role: "viewer" }]);
    renderPagina();
    await screen.findByText("Carlos");

    expect(screen.getByText("Dono")).toHaveClass("tt-role--owner");
    expect(screen.getByText("Membro")).toHaveClass("tt-role");
    expect(screen.getByText("Viewer")).toHaveClass("tt-role");
    expect(screen.getByText("Membro")).not.toHaveClass("tt-role--owner");
  });

  // CA-05 — single workspace: NENHUM seletor de workspace ativo. Vista de
  // member/viewer pra não trazer os selects de gestão do owner.
  it("com 1 workspace não existe seletor", async () => {
    setMe([{ id: "w1", name: "Workspace de Ana", role: "viewer" }]);
    renderPagina();
    await screen.findByText("Carlos");

    expect(screen.queryByRole("combobox")).toBeNull();
  });

  // CA-09 — viewer vê a página igual (leitura, sem gating)
  it("viewer renderiza a página normalmente", async () => {
    setMe([{ id: "w1", name: "Workspace de Ana", role: "viewer" }]);
    renderPagina();

    expect(await screen.findByText("Carlos")).toBeInTheDocument();
    expect(screen.getByTestId("workspace-name")).toBeInTheDocument();
  });

  // fase 22 T-07 (CA-10): a linha de membro usa a classe responsiva .tt-ws-member
  // (≥ md altura fixa; < md quebra + altura auto pra os controles não cortarem
  // nem estourarem). Estrutura (jsdom) — overflow a 375px = T-10/CA-19.
  it("a linha de membro tem a classe responsiva .tt-ws-member", async () => {
    setMe([{ id: "w1", name: "Workspace de Ana", role: "viewer" }]);
    renderPagina();
    await screen.findByText("Carlos");
    expect(screen.getByTestId("member-u2")).toHaveClass("tt-ws-member");
  });
});

describe("página Workspace — seletor de ativo (T-03)", () => {
  // member nos dois: foca o seletor de workspace ATIVO sem trazer os selects
  // de gestão do owner (que também são comboboxes).
  const doisWs = [
    { id: "w1", name: "Workspace de Ana", role: "member" as const },
    { id: "w2", name: "Time Primor", role: "member" as const },
  ];

  // CA-06 — seletor com >1; trocar muda o ativo em TODO o app e persiste
  it("com 2 workspaces o seletor aparece e a troca vale pro app todo", async () => {
    setMe(doisWs);
    renderPagina();
    await screen.findByText("Carlos");

    const seletor = screen.getByRole("combobox", { name: /workspace ativo/i });
    expect(seletor).toBeInTheDocument();
    expect(screen.getByTestId("probe-active").textContent).toBe("w1");

    fireEvent.change(seletor, { target: { value: "w2" } });

    expect(screen.getByTestId("probe-active").textContent).toBe("w2");
    expect(localStorage.getItem("tt.ws.active")).toBe("w2");
  });

  // CA-08 — trocar invalida o cache e re-busca o detalhe do ws NOVO
  it("trocar invalida as queries e busca o workspace novo", async () => {
    setMe(doisWs);
    renderPagina();
    await screen.findByText("Carlos");
    const invalidateSpy = vi.spyOn(qc, "invalidateQueries");

    fireEvent.change(
      screen.getByRole("combobox", { name: /workspace ativo/i }),
      {
        target: { value: "w2" },
      },
    );

    expect(invalidateSpy).toHaveBeenCalled();
    await waitFor(() => expect(api.getWorkspace).toHaveBeenCalledWith("w2"));
    expect(await screen.findByTestId("workspace-name")).toHaveTextContent(
      "Time Primor",
    );
  });
});
