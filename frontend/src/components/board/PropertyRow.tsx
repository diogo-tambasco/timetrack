// PropertyRow (fase 25 T-10, CA-21/22): uma linha de propriedade CUSTOM no popup
// central. prop-k = nome da def (editável → patchProperty no blur); prop-v = o
// editor de value por tipo (reusa PropertyCell); ✕ = deleteProperty (remove a def do
// PROJETO inteiro, com confirmação). Gating por canWrite (read-only sem ações).
import { useState } from "react";
import type { BoardTask, PropertyDef } from "../../lib/api";
import { Icon } from "../icons";
import {
  usePatchProperty,
  useDeleteProperty,
} from "../../lib/queries/properties";
import PropertyCell from "./PropertyCell";

export default function PropertyRow({
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
  const patchDef = usePatchProperty(wsId, projectId);
  const deleteDef = useDeleteProperty(wsId, projectId);
  const [name, setName] = useState(def.name);

  const renameOnBlur = () => {
    const v = name.trim();
    if (v === "" || v === def.name) {
      setName(def.name);
      return;
    }
    patchDef.mutate({ id: def.id, patch: { name: v } });
  };

  const removeDef = () => {
    if (typeof window !== "undefined" && !window.confirm(`Remover a propriedade "${def.name}" do projeto?`)) {
      return;
    }
    deleteDef.mutate(def.id);
  };

  return (
    <div className="prop-row" style={{ display: "flex", alignItems: "center", gap: 10, minHeight: 33, borderRadius: 7, padding: "0 6px" }}>
      {canWrite ? (
        <span style={{ width: 150, flex: "0 0 auto", display: "flex", alignItems: "center", gap: 8 }}>
          <span style={{ color: "var(--text-3)", display: "inline-flex", flex: "0 0 auto" }}>
            <Icon.dots size={13} />
          </span>
          <input
            className="prop-k"
            aria-label={`Nome da propriedade ${def.name}`}
            value={name}
            onChange={(e) => setName(e.target.value)}
            onBlur={renameOnBlur}
            onKeyDown={(e) => e.key === "Enter" && (e.target as HTMLInputElement).blur()}
            style={{
              flex: 1,
              minWidth: 0,
              background: "transparent",
              border: "none",
              outline: "none",
              color: "var(--text-3)",
              font: "inherit",
              fontSize: 12.5,
            }}
          />
        </span>
      ) : (
        <span className="prop-k faint" style={{ fontSize: 12.5, width: 150, flex: "0 0 auto", display: "flex", alignItems: "center", gap: 8 }}>
          <Icon.dots size={13} />
          {def.name}
        </span>
      )}

      <div className="prop-v" style={{ flex: 1, minWidth: 0 }}>
        <PropertyCell wsId={wsId} projectId={projectId} def={def} task={task} canWrite={canWrite} />
      </div>

      {canWrite && (
        <button
          type="button"
          aria-label={`Remover propriedade ${def.name}`}
          onClick={removeDef}
          className="tt-iconbtn"
          style={{ flex: "0 0 auto", background: "transparent", border: "none", cursor: "pointer", color: "var(--text-3)" }}
        >
          <Icon.x size={13} />
        </button>
      )}
    </div>
  );
}
