import { describe, it, expect } from "vitest";
import {
  validateValue,
  isISODate,
  resolveOption,
  emptyValue,
  valueOf,
} from "./propertyValue";
import type { PropertyDef } from "./api";

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

describe("propertyValue.validateValue", () => {
  it("text/url: string ≤2000", () => {
    expect(validateValue("text", "oi")).toBe(true);
    expect(validateValue("text", "")).toBe(true);
    expect(validateValue("text", "x".repeat(2001))).toBe(false);
    expect(validateValue("url", "https://x")).toBe(true);
    expect(validateValue("text", 5)).toBe(false);
  });
  it("number: finito (rejeita string e NaN)", () => {
    expect(validateValue("number", 12.5)).toBe(true);
    expect(validateValue("number", -3)).toBe(true);
    expect(validateValue("number", "12")).toBe(false);
    expect(validateValue("number", NaN)).toBe(false);
  });
  it("date: YYYY-MM-DD de calendário real", () => {
    expect(validateValue("date", "2026-07-01")).toBe(true);
    expect(validateValue("date", "2026-13-40")).toBe(false);
    expect(validateValue("date", "ontem")).toBe(false);
  });
  it("checkbox: boolean", () => {
    expect(validateValue("checkbox", true)).toBe(true);
    expect(validateValue("checkbox", false)).toBe(true);
    expect(validateValue("checkbox", 1)).toBe(false);
  });
  it("select: string (o backend valida o id)", () => {
    expect(validateValue("select", "op_a")).toBe(true);
    expect(validateValue("select", 1)).toBe(false);
  });
});

describe("propertyValue.isISODate", () => {
  it("aceita real, rejeita impossível", () => {
    expect(isISODate("2026-02-28")).toBe(true);
    expect(isISODate("2026-02-30")).toBe(false);
    expect(isISODate("2026-13-01")).toBe(false);
    expect(isISODate("2026-7-1")).toBe(false);
  });
});

describe("propertyValue.resolveOption", () => {
  it("resolve id existente", () => {
    expect(resolveOption(selDef, "op_b")?.label).toBe("Arkan");
  });
  it("id órfão → null (CA-13)", () => {
    expect(resolveOption(selDef, "op_z")).toBeNull();
    expect(resolveOption(selDef, 5)).toBeNull();
  });
});

describe("propertyValue.emptyValue / valueOf", () => {
  it("emptyValue por tipo", () => {
    expect(emptyValue("checkbox")).toBe(false);
    expect(emptyValue("number")).toBeNull();
    expect(emptyValue("text")).toBe("");
  });
  it("valueOf extrai por def_id", () => {
    const vals = [{ def_id: "d1", value: "op_a" }];
    expect(valueOf(vals, "d1")).toBe("op_a");
    expect(valueOf(vals, "d2")).toBeUndefined();
  });
});
