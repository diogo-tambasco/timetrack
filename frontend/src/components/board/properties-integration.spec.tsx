import { describe, it, expect, vi } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import TaskTableView from "./TaskTableView";
import CustomProperties from "./CustomProperties";
import { mkTask } from "./testFixtures";
import type { PropertyDef } from "../../lib/api";

vi.mock("../../lib/api", async (importOriginal) => {
  const mod = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...mod,
    setPropertyValue: vi.fn().mockResolvedValue(undefined),
    clearPropertyValue: vi.fn().mockResolvedValue(undefined),
    createProperty: vi.fn().mockResolvedValue({ id: "x" }),
    patchProperty: vi.fn().mockResolvedValue({}),
    deleteProperty: vi.fn().mockResolvedValue(undefined),
  };
});

const selDef: PropertyDef = {
  id: "d1",
  project_id: "p1",
  workspace_id: "w1",
  name: "Cliente",
  type: "select",
  options: [
    { id: "op_a", label: "Lauda", color: null },
    { id: "op_b", label: "Arkan", color: "#4f9dde" },
  ],
  position: 0,
};

function wrap(node: React.ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={qc}>{node}</QueryClientProvider>);
}

describe("TaskTableView colunas custom (CA-19)", () => {
  it("renderiza coluna por def com o value da task", () => {
    const task = mkTask({ id: "t1", name: "T", property_values: [{ def_id: "d1", value: "op_a" }] });
    wrap(
      <TaskTableView
        tasks={[task]}
        canWrite
        onOpenTask={vi.fn()}
        onQuickAdd={vi.fn()}
        onCycleStatus={vi.fn()}
        onCyclePriority={vi.fn()}
        wsId="w1"
        projectId="p1"
        defs={[selDef]}
      />,
    );
    // coluna "Cliente" no header + célula select com a option selecionada
    expect(screen.getByRole("columnheader", { name: "Cliente" })).toBeTruthy();
    expect((screen.getByLabelText("Cliente") as HTMLSelectElement).value).toBe("op_a");
  });

  it("sem defs: só as colunas nativas (compat fase 24)", () => {
    wrap(
      <TaskTableView
        tasks={[mkTask({ id: "t1" })]}
        canWrite
        onOpenTask={vi.fn()}
        onQuickAdd={vi.fn()}
        onCycleStatus={vi.fn()}
        onCyclePriority={vi.fn()}
      />,
    );
    expect(screen.queryByRole("columnheader", { name: "Cliente" })).toBeNull();
  });
});

describe("CustomProperties no popup (CA-21/22)", () => {
  it("lista uma PropertyRow por def + Add property (canWrite)", () => {
    wrap(
      <CustomProperties
        wsId="w1"
        projectId="p1"
        task={mkTask({ id: "t1" })}
        defs={[selDef]}
        canWrite
      />,
    );
    expect(screen.getByLabelText("Nome da propriedade Cliente")).toBeTruthy();
    expect(screen.getByLabelText("Add property")).toBeTruthy();
  });

  it("canWrite=false sem defs: bloco some", () => {
    const { container } = wrap(
      <CustomProperties wsId="w1" projectId="p1" task={mkTask({ id: "t1" })} defs={[]} canWrite={false} />,
    );
    expect(container.querySelector('[data-testid="custom-properties"]')).toBeNull();
  });

  it("canWrite=false com defs: linha só-leitura, sem Add (CA-22)", () => {
    wrap(
      <CustomProperties wsId="w1" projectId="p1" task={mkTask({ id: "t1" })} defs={[selDef]} canWrite={false} />,
    );
    expect(screen.getByText("Cliente")).toBeTruthy();
    expect(screen.queryByLabelText("Add property")).toBeNull();
  });
});

// RNF-03: a Table reusa a classe real .tt-table; as linhas do popup usam .prop-k/.prop-v
// (porte do mockup pro CSS real — check-cores não detecta classe AUSENTE).
describe("presença das classes CSS (RNF-03)", () => {
  it("tokens.css tem .tt-table, .prop-k e .prop-v", () => {
    const css = readFileSync(path.resolve(process.cwd(), "src/styles/tokens.css"), "utf8");
    expect(css).toContain(".tt-table");
    expect(css).toContain(".prop-k");
    expect(css).toContain(".prop-v");
  });
});
