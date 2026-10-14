import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import PropertyRow from "./PropertyRow";
import AddPropertyButton from "./AddPropertyButton";
import { mkTask } from "./testFixtures";
import type { PropertyDef } from "../../lib/api";
import * as api from "../../lib/api";

vi.mock("../../lib/api", async (importOriginal) => {
  const mod = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...mod,
    patchProperty: vi.fn().mockResolvedValue({}),
    deleteProperty: vi.fn().mockResolvedValue(undefined),
    createProperty: vi.fn().mockResolvedValue({ id: "dnew" }),
  };
});

const def: PropertyDef = {
  id: "d1",
  project_id: "p1",
  workspace_id: "w1",
  name: "Cliente",
  type: "text",
  options: [],
  position: 0,
};

function wrap(node: React.ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={qc}>{node}</QueryClientProvider>);
}

beforeEach(() => vi.clearAllMocks());

describe("PropertyRow (CA-21/22)", () => {
  it("renomear chama patchProperty", async () => {
    wrap(<PropertyRow wsId="w1" projectId="p1" def={def} task={mkTask({ id: "t1" })} canWrite />);
    const input = screen.getByLabelText("Nome da propriedade Cliente");
    fireEvent.change(input, { target: { value: "Conta" } });
    fireEvent.blur(input);
    await waitFor(() =>
      expect(api.patchProperty).toHaveBeenCalledWith("w1", "p1", "d1", { name: "Conta" }),
    );
  });

  it("✕ com confirmação chama deleteProperty (remove do projeto)", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    wrap(<PropertyRow wsId="w1" projectId="p1" def={def} task={mkTask({ id: "t1" })} canWrite />);
    fireEvent.click(screen.getByLabelText("Remover propriedade Cliente"));
    await waitFor(() => expect(api.deleteProperty).toHaveBeenCalledWith("w1", "p1", "d1"));
  });

  it("✕ cancelado NÃO deleta", () => {
    vi.spyOn(window, "confirm").mockReturnValue(false);
    wrap(<PropertyRow wsId="w1" projectId="p1" def={def} task={mkTask({ id: "t1" })} canWrite />);
    fireEvent.click(screen.getByLabelText("Remover propriedade Cliente"));
    expect(api.deleteProperty).not.toHaveBeenCalled();
  });

  it("canWrite=false: sem input de nome nem ✕ (CA-22)", () => {
    wrap(<PropertyRow wsId="w1" projectId="p1" def={def} task={mkTask({ id: "t1" })} canWrite={false} />);
    expect(screen.queryByLabelText("Nome da propriedade Cliente")).toBeNull();
    expect(screen.queryByLabelText("Remover propriedade Cliente")).toBeNull();
    expect(screen.getByText("Cliente")).toBeTruthy();
  });
});

describe("AddPropertyButton (CA-21/23, Notion-style fase 31)", () => {
  it("abre o popover e cria def text (tipo default)", async () => {
    wrap(<AddPropertyButton wsId="w1" projectId="p1" canWrite />);
    fireEvent.click(screen.getByLabelText("Add property"));
    fireEvent.change(screen.getByLabelText("Nome da nova propriedade"), { target: { value: "Estimativa" } });
    fireEvent.click(screen.getByRole("option", { name: "Número" })); // escolhe o tipo na lista
    fireEvent.click(screen.getByLabelText("Criar propriedade"));
    await waitFor(() =>
      expect(api.createProperty).toHaveBeenCalledWith("w1", "p1", { name: "Estimativa", type: "number" }),
    );
  });

  it("cria select com 2 opções coloridas (CA-23)", async () => {
    wrap(<AddPropertyButton wsId="w1" projectId="p1" canWrite />);
    fireEvent.click(screen.getByLabelText("Add property"));
    fireEvent.change(screen.getByLabelText("Nome da nova propriedade"), { target: { value: "Cliente" } });
    fireEvent.click(screen.getByRole("option", { name: "Seleção" }));
    fireEvent.change(screen.getByLabelText("Opção 1"), { target: { value: "Lauda" } });
    fireEvent.change(screen.getByLabelText("Opção 2"), { target: { value: "Arkan" } });
    fireEvent.click(screen.getByLabelText("Criar propriedade"));
    await waitFor(() =>
      expect(api.createProperty).toHaveBeenCalledWith(
        "w1",
        "p1",
        expect.objectContaining({
          name: "Cliente",
          type: "select",
          options: [
            expect.objectContaining({ label: "Lauda" }),
            expect.objectContaining({ label: "Arkan" }),
          ],
        }),
      ),
    );
  });

  it("nome vazio não cria (botão Criar desabilitado)", () => {
    wrap(<AddPropertyButton wsId="w1" projectId="p1" canWrite />);
    fireEvent.click(screen.getByLabelText("Add property"));
    fireEvent.click(screen.getByLabelText("Criar propriedade"));
    expect(api.createProperty).not.toHaveBeenCalled();
  });

  it("select SEM opções não cria (CA-03 — T-14 #7)", async () => {
    wrap(<AddPropertyButton wsId="w1" projectId="p1" canWrite />);
    fireEvent.click(screen.getByLabelText("Add property"));
    fireEvent.change(screen.getByLabelText("Nome da nova propriedade"), { target: { value: "Cliente" } });
    fireEvent.click(screen.getByRole("option", { name: "Seleção" }));
    // opções vazias → Criar não dispara. Flush dos microtasks antes de afirmar.
    fireEvent.click(screen.getByLabelText("Criar propriedade"));
    await new Promise((r) => setTimeout(r, 20));
    expect(api.createProperty).not.toHaveBeenCalled();
  });

  it("canWrite=false: some (CA-22)", () => {
    wrap(<AddPropertyButton wsId="w1" projectId="p1" canWrite={false} />);
    expect(screen.queryByLabelText("Add property")).toBeNull();
  });
});
