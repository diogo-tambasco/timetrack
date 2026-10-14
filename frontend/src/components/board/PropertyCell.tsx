// PropertyCell (fase 25 T-09, CA-19/20/22): edita inline o value de uma def numa
// task, por tipo (text/url/number = input; date = date-picker; checkbox = toggle;
// select = dropdown das options com a cor). Esvaziar → clearValue; senão setValue.
// Gating por canWrite (read-only quando false). O hook invalida o boardKey → reflete
// sem reload. number/date inválido é barrado no client (o backend é a autoridade).
import { useState } from "react";
import type { BoardTask, PropertyDef } from "../../lib/api";
import {
  useSetPropertyValue,
  useClearPropertyValue,
} from "../../lib/queries/properties";
import { valueOf, validateValue } from "../../lib/propertyValue";

const inputStyle: React.CSSProperties = {
  width: "100%",
  minWidth: 80,
  background: "var(--surface-1)",
  border: "1px solid var(--border)",
  borderRadius: 6,
  padding: "4px 7px",
  fontFamily: "var(--font-ui)",
  fontSize: 12.5,
  color: "var(--text-1)",
};

export default function PropertyCell({
  wsId,
  projectId,
  def,
  task,
  canWrite,
}: {
  wsId: string;
  projectId: string;
  def: PropertyDef;
  task: BoardTask;
  canWrite: boolean;
}) {
  const setVal = useSetPropertyValue(wsId, projectId);
  const clearVal = useClearPropertyValue(wsId, projectId);
  const current = valueOf(task.property_values, def.id);

  const set = (value: unknown) =>
    setVal.mutate({ taskId: task.id, defId: def.id, value });
  const clear = () => clearVal.mutate({ taskId: task.id, defId: def.id });

  // ── checkbox ──────────────────────────────────────────────────────────────
  if (def.type === "checkbox") {
    return (
      <input
        type="checkbox"
        aria-label={def.name}
        checked={current === true}
        disabled={!canWrite}
        onChange={(e) => canWrite && set(e.target.checked)}
      />
    );
  }

  // ── select ────────────────────────────────────────────────────────────────
  if (def.type === "select") {
    const selected = typeof current === "string" ? current : "";
    const opt = def.options.find((o) => o.id === selected) ?? null;
    return (
      <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
        {opt?.color != null && (
          <span
            aria-hidden
            style={{
              width: 8,
              height: 8,
              borderRadius: 999,
              background: opt.color,
              flex: "none",
            }}
          />
        )}
        <select
          aria-label={def.name}
          value={selected}
          disabled={!canWrite}
          onChange={(e) => {
            if (!canWrite) return;
            e.target.value === "" ? clear() : set(e.target.value);
          }}
          style={{ ...inputStyle, minWidth: 100 }}
        >
          <option value="">—</option>
          {def.options.map((o) => (
            <option key={o.id} value={o.id}>
              {o.label}
            </option>
          ))}
        </select>
      </div>
    );
  }

  // ── date ──────────────────────────────────────────────────────────────────
  if (def.type === "date") {
    const v = typeof current === "string" ? current : "";
    return (
      <input
        type="date"
        aria-label={def.name}
        value={v}
        disabled={!canWrite}
        onChange={(e) => {
          if (!canWrite) return;
          const next = e.target.value;
          next === "" ? clear() : validateValue("date", next) && set(next);
        }}
        style={inputStyle}
      />
    );
  }

  // ── text / url / number (input com commit no blur/Enter) ───────────────────
  return (
    <TextLikeCell
      def={def}
      current={current}
      canWrite={canWrite}
      onSet={set}
      onClear={clear}
    />
  );
}

function TextLikeCell({
  def,
  current,
  canWrite,
  onSet,
  onClear,
}: {
  def: PropertyDef;
  current: unknown;
  canWrite: boolean;
  onSet: (v: unknown) => void;
  onClear: () => void;
}) {
  const initial =
    current == null ? "" : typeof current === "number" ? String(current) : String(current);
  const [draft, setDraft] = useState(initial);

  const commit = () => {
    if (!canWrite) return;
    const raw = draft.trim();
    if (raw === "") {
      if (current != null) onClear(); // esvaziou → clear (CA-20)
      return;
    }
    if (def.type === "number") {
      const n = Number(raw);
      if (!validateValue("number", n)) return; // barra no client (backend é autoridade)
      if (n !== current) onSet(n);
      return;
    }
    if (raw !== current) onSet(raw);
  };

  if (!canWrite) {
    return <span style={{ color: "var(--text-1)" }}>{initial || "—"}</span>;
  }
  return (
    <input
      type={def.type === "number" ? "number" : "text"}
      aria-label={def.name}
      value={draft}
      onChange={(e) => setDraft(e.target.value)}
      onBlur={commit}
      onKeyDown={(e) => {
        if (e.key === "Enter") {
          e.preventDefault();
          (e.target as HTMLInputElement).blur();
        }
      }}
      style={inputStyle}
    />
  );
}
