// propertyValue.ts — fase 25: helpers PUROS de value de propriedade (testáveis em
// jsdom). validateValue espelha o backend (CA-09, autoridade final é o servidor);
// resolveOption resolve o id→option de um select (id órfão → null, CA-13);
// emptyValue dá o "vazio" por tipo (a célula decide clear vs set).
import type { PropertyDef, PropertyOption, PropertyType } from "./api";

// validateValue: true se v casa com o type (validação leve do cliente — number/date
// barrados antes do PUT; o backend revalida). null/"" são tratados como "vazio"
// (limpar) pela célula, não por aqui.
export function validateValue(type: PropertyType, v: unknown): boolean {
  switch (type) {
    case "text":
    case "url":
      return typeof v === "string" && v.length <= 2000;
    case "number":
      return typeof v === "number" && Number.isFinite(v);
    case "date":
      return typeof v === "string" && isISODate(v);
    case "checkbox":
      return typeof v === "boolean";
    case "select":
      return typeof v === "string";
    default:
      return false;
  }
}

// isISODate: "YYYY-MM-DD" de calendário real (rejeita 2026-13-40).
export function isISODate(s: string): boolean {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s);
  if (!m) return false;
  const [, y, mo, d] = m;
  const dt = new Date(`${y}-${mo}-${d}T00:00:00Z`);
  return (
    dt.getUTCFullYear() === Number(y) &&
    dt.getUTCMonth() + 1 === Number(mo) &&
    dt.getUTCDate() === Number(d)
  );
}

// resolveOption: o option de def.options cujo id === o value; null se o id sumiu
// (option removido — CA-13) ou se o value não é string.
export function resolveOption(
  def: PropertyDef,
  value: unknown,
): PropertyOption | null {
  if (typeof value !== "string") return null;
  return def.options.find((o) => o.id === value) ?? null;
}

// emptyValue: o "vazio" inicial por tipo (pro editor). checkbox = false; o resto =
// "" (string vazia) ou null — a célula manda DELETE/value:null pra limpar.
export function emptyValue(type: PropertyType): unknown {
  if (type === "checkbox") return false;
  if (type === "number") return null;
  return "";
}

// valueOf: extrai o value (cru) de uma task pro defId, ou undefined se não houver.
export function valueOf(
  values: { def_id: string; value: unknown }[],
  defId: string,
): unknown {
  return values.find((v) => v.def_id === defId)?.value;
}
