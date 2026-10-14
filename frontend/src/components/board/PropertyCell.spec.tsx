import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import PropertyCell from "./PropertyCell";
import { mkTask } from "./testFixtures";
import type { PropertyDef } from "../../lib/api";
import * as api from "../../lib/api";

vi.mock("../../lib/api", async (importOriginal) => {
  const mod = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...mod,
    setPropertyValue: vi.fn().mockResolvedValue({ def_id: "d", value: null }),
    clearPropertyValue: vi.fn().mockResolvedValue(undefined),
  };
});

function def(over: Partial<PropertyDef>): PropertyDef {
  return {
    id: "d1",
    project_id: "p1",
    workspace_id: "w1",
    name: "Campo",
    type: "text",
    options: [],
    position: 0,
    ...over,
  };
}

function renderCell(d: PropertyDef, canWrite: boolean, values: { def_id: string; value: unknown }[] = []) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <PropertyCell
        wsId="w1"
        projectId="p1"
        def={d}
        task={mkTask({ id: "t1", property_values: values })}
        canWrite={canWrite}
      />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("PropertyCell (CA-19/20/22)", () => {
  it("text: digitar + blur chama setPropertyValue", async () => {
    renderCell(def({ type: "text" }), true);
    const input = screen.getByLabelText("Campo");
    fireEvent.change(input, { target: { value: "oi" } });
    fireEvent.blur(input);
    await waitFor(() =>
      expect(api.setPropertyValue).toHaveBeenCalledWith("w1", "p1", "t1", "d1", "oi"),
    );
  });

  it("text: esvaziar chama clearPropertyValue (CA-20)", async () => {
    renderCell(def({ type: "text" }), true, [{ def_id: "d1", value: "oi" }]);
    const input = screen.getByLabelText("Campo");
    fireEvent.change(input, { target: { value: "" } });
    fireEvent.blur(input);
    await waitFor(() =>
      expect(api.clearPropertyValue).toHaveBeenCalledWith("w1", "p1", "t1", "d1"),
    );
    expect(api.setPropertyValue).not.toHaveBeenCalled();
  });

  it("number: envia número (não string)", async () => {
    renderCell(def({ type: "number" }), true);
    const input = screen.getByLabelText("Campo");
    fireEvent.change(input, { target: { value: "12.5" } });
    fireEvent.blur(input);
    await waitFor(() =>
      expect(api.setPropertyValue).toHaveBeenCalledWith("w1", "p1", "t1", "d1", 12.5),
    );
  });

  it("checkbox: marcar envia bool true", async () => {
    renderCell(def({ type: "checkbox" }), true);
    fireEvent.click(screen.getByLabelText("Campo"));
    await waitFor(() =>
      expect(api.setPropertyValue).toHaveBeenCalledWith("w1", "p1", "t1", "d1", true),
    );
  });

  it("checkbox: DESMARCAR envia bool false (T-14 #4)", async () => {
    renderCell(def({ type: "checkbox" }), true, [{ def_id: "d1", value: true }]);
    const cb = screen.getByLabelText("Campo") as HTMLInputElement;
    expect(cb.checked).toBe(true);
    fireEvent.click(cb);
    await waitFor(() =>
      expect(api.setPropertyValue).toHaveBeenCalledWith("w1", "p1", "t1", "d1", false),
    );
  });

  it("select: escolher '—' (vazio) chama clearPropertyValue (T-14 #5)", async () => {
    const d = def({ type: "select", options: [{ id: "op_a", label: "Lauda", color: null }] });
    renderCell(d, true, [{ def_id: "d1", value: "op_a" }]);
    fireEvent.change(screen.getByLabelText("Campo"), { target: { value: "" } });
    await waitFor(() =>
      expect(api.clearPropertyValue).toHaveBeenCalledWith("w1", "p1", "t1", "d1"),
    );
    expect(api.setPropertyValue).not.toHaveBeenCalled();
  });

  // T-14 #6 (date inválido barrado no client) NÃO é testável via DOM: o
  // <input type="date"> do jsdom (e dos browsers) normaliza valor inválido pra "" antes
  // do onChange, então o ramo `validateValue("date", next) && set(next)` é inalcançável
  // por um valor não-vazio inválido. A regra de validação está coberta em
  // propertyValue.test.ts (validateValue "date" rejeita "2026-13-40"). Gap registrado.

  it("date: escolher envia ISO", async () => {
    renderCell(def({ type: "date" }), true);
    fireEvent.change(screen.getByLabelText("Campo"), { target: { value: "2026-07-01" } });
    await waitFor(() =>
      expect(api.setPropertyValue).toHaveBeenCalledWith("w1", "p1", "t1", "d1", "2026-07-01"),
    );
  });

  it("select: escolher option envia o id", async () => {
    const d = def({
      type: "select",
      options: [
        { id: "op_a", label: "Lauda", color: null },
        { id: "op_b", label: "Arkan", color: "#4f9dde" },
      ],
    });
    renderCell(d, true);
    fireEvent.change(screen.getByLabelText("Campo"), { target: { value: "op_b" } });
    await waitFor(() =>
      expect(api.setPropertyValue).toHaveBeenCalledWith("w1", "p1", "t1", "d1", "op_b"),
    );
  });

  it("select: value órfão de option → mostra vazio (CA-13)", () => {
    const d = def({ type: "select", options: [{ id: "op_a", label: "Lauda", color: null }] });
    renderCell(d, true, [{ def_id: "d1", value: "op_zzz" }]);
    expect((screen.getByLabelText("Campo") as HTMLSelectElement).value).toBe("");
  });

  it("canWrite=false: text só-leitura (sem input editável)", () => {
    renderCell(def({ type: "text" }), false, [{ def_id: "d1", value: "valor" }]);
    expect(screen.queryByLabelText("Campo")).toBeNull();
    expect(screen.getByText("valor")).toBeTruthy();
  });

  it("canWrite=false: checkbox desabilitado, não chama set", () => {
    renderCell(def({ type: "checkbox" }), false);
    const cb = screen.getByLabelText("Campo") as HTMLInputElement;
    expect(cb.disabled).toBe(true);
    fireEvent.click(cb);
    expect(api.setPropertyValue).not.toHaveBeenCalled();
  });
});
