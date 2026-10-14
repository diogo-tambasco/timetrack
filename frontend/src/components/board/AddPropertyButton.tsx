// AddPropertyButton (fase 25 / fase 31): cria uma def custom no projeto, num
// POPOVER estilo Notion — nome + lista de tipos (ícone · rótulo · descrição,
// selecionável) + editor de opções coloridas quando "Seleção". O `type` é IMUTÁVEL
// após criar (D-04), por isso nomear+tipar ANTES de criar. Some quando
// canWrite=false. Ao criar, o board invalida e a linha aparece.
import { useRef, useState } from "react";
import type { CreatePropertyInput, PropertyType } from "../../lib/api";
import { useCreateProperty } from "../../lib/queries/properties";
import { Icon } from "../icons";
import AnchoredPopover from "./AnchoredPopover";
import { PROJECT_PALETTE } from "../../lib/palette";

const TYPES: { v: PropertyType; label: string; desc: string; Glyph: typeof Icon.doc }[] = [
  { v: "text", label: "Texto", desc: "Texto livre", Glyph: Icon.doc },
  { v: "number", label: "Número", desc: "Um valor numérico", Glyph: Icon.chart },
  { v: "select", label: "Seleção", desc: "Uma opção de uma lista", Glyph: Icon.list },
  { v: "date", label: "Data", desc: "Uma data", Glyph: Icon.cal },
  { v: "url", label: "URL", desc: "Um link", Glyph: Icon.link },
  { v: "checkbox", label: "Checkbox", desc: "Marcado ou não", Glyph: Icon.check },
];

export default function AddPropertyButton({
  wsId,
  projectId,
  canWrite,
}: {
  wsId: string;
  projectId: string;
  canWrite: boolean;
}) {
  const createDef = useCreateProperty(wsId, projectId);
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [type, setType] = useState<PropertyType>("text");
  const [options, setOptions] = useState<string[]>(["", ""]);
  const anchorRef = useRef<HTMLButtonElement>(null);
  const lastOptRef = useRef<HTMLInputElement>(null);

  if (!canWrite) return null;

  // UM campo só (sem searchbox separado): o nome TAMBÉM filtra os tipos. Se o que
  // você digitou casa com algum tipo → mostra os que casam; senão (ex.: "Cliente")
  // → mostra TODOS pra escolher. Sem layout shift: lista em container de altura fixa.
  const norm = name.trim().toLowerCase();
  const matched = norm ? TYPES.filter((t) => `${t.label} ${t.desc}`.toLowerCase().includes(norm)) : TYPES;
  const shown = matched.length ? matched : TYPES;

  const reset = () => {
    setOpen(false);
    setName("");
    setType("text");
    setOptions(["", ""]);
  };

  const validOptions = options.map((o) => o.trim()).filter(Boolean);
  const canCreate =
    name.trim() !== "" && !createDef.isPending && (type !== "select" || validOptions.length > 0);

  const submit = () => {
    if (!canCreate) return;
    const input: CreatePropertyInput = { name: name.trim(), type };
    if (type === "select") {
      input.options = validOptions.map((label, i) => ({
        label,
        color: PROJECT_PALETTE[i % PROJECT_PALETTE.length],
      }));
    }
    createDef.mutate(input, { onSuccess: reset });
  };

  const setOpt = (i: number, v: string) => setOptions((o) => o.map((x, j) => (j === i ? v : x)));
  const addOpt = () => setOptions((o) => [...o, ""]);
  const removeOpt = (i: number) => setOptions((o) => (o.length > 1 ? o.filter((_, j) => j !== i) : o));

  return (
    <>
      <button
        ref={anchorRef}
        type="button"
        className="tt-btn tt-btn--ghost"
        aria-label="Add property"
        onClick={() => setOpen(true)}
        style={{ alignSelf: "flex-start", padding: "5px 8px", fontSize: 11.5, gap: 5 }}
      >
        <Icon.plus size={13} sw={2.2} />
        Add property
      </button>

      {open && (
        <AnchoredPopover anchorRef={anchorRef} onClose={reset}>
          <div
            role="dialog"
            aria-label="Nova propriedade"
            style={{
              width: 296,
              maxHeight: "min(72vh, 560px)",
              overflowY: "auto",
              background: "var(--surface-1)",
              border: "1px solid var(--border-2)",
              borderRadius: 12,
              boxShadow: "var(--shadow)",
              padding: 10,
              display: "flex",
              flexDirection: "column",
              gap: 10,
            }}
          >
            <input
              aria-label="Nome da nova propriedade"
              autoFocus
              value={name}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && type !== "select" && submit()}
              placeholder="Nome da propriedade"
              style={{
                background: "var(--surface-2)",
                border: "1px solid var(--border-2)",
                borderRadius: 8,
                padding: "8px 10px",
                fontFamily: "var(--font-ui)",
                fontSize: 13.5,
                fontWeight: 540,
                color: "var(--text-1)",
                outline: "none",
                width: "100%",
              }}
            />

            <div className="eyebrow" style={{ marginTop: 2 }}>
              Tipo
            </div>
            <div
              role="listbox"
              aria-label="Tipo da propriedade"
              style={{ display: "flex", flexDirection: "column", gap: 1, minHeight: 260 }}
            >
              {shown.map(({ v, label, desc, Glyph }) => {
                const sel = type === v;
                return (
                  <button
                    key={v}
                    type="button"
                    role="option"
                    aria-selected={sel}
                    aria-label={label}
                    onClick={() => setType(v)}
                    style={{
                      display: "flex",
                      alignItems: "center",
                      gap: 11,
                      width: "100%",
                      padding: "8px 9px",
                      borderRadius: 8,
                      border: "none",
                      background: sel ? "var(--surface-3)" : "transparent",
                      color: "var(--text-1)",
                      font: "inherit",
                      cursor: "pointer",
                      textAlign: "left",
                    }}
                  >
                    <span
                      aria-hidden
                      style={{
                        width: 28,
                        height: 28,
                        borderRadius: 7,
                        background: "var(--surface-2)",
                        border: "1px solid var(--border)",
                        display: "grid",
                        placeItems: "center",
                        color: sel ? "var(--accent)" : "var(--text-3)",
                        flex: "0 0 auto",
                      }}
                    >
                      <Glyph size={15} />
                    </span>
                    <span style={{ flex: 1, minWidth: 0 }}>
                      <span style={{ display: "block", fontSize: 13, fontWeight: 540 }}>{label}</span>
                      <span className="faint" style={{ display: "block", fontSize: 11 }}>
                        {desc}
                      </span>
                    </span>
                    {sel && <Icon.check size={15} sw={2.4} style={{ color: "var(--accent)", flex: "0 0 auto" }} />}
                  </button>
                );
              })}
            </div>

            {type === "select" && (
              <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                <div className="eyebrow" style={{ marginTop: 2 }}>
                  Opções
                </div>
                {options.map((o, i) => (
                  <div key={i} style={{ display: "flex", alignItems: "center", gap: 8 }}>
                    <span
                      aria-hidden
                      style={{
                        width: 11,
                        height: 11,
                        borderRadius: "50%",
                        background: PROJECT_PALETTE[i % PROJECT_PALETTE.length],
                        flex: "0 0 auto",
                      }}
                    />
                    <input
                      ref={i === options.length - 1 ? lastOptRef : undefined}
                      aria-label={`Opção ${i + 1}`}
                      value={o}
                      onChange={(e) => setOpt(i, e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") {
                          e.preventDefault();
                          if (i === options.length - 1 && o.trim() !== "") {
                            addOpt();
                            requestAnimationFrame(() => lastOptRef.current?.focus());
                          } else {
                            submit();
                          }
                        }
                      }}
                      placeholder={`Opção ${i + 1}`}
                      style={{
                        flex: 1,
                        minWidth: 0,
                        background: "var(--surface-2)",
                        border: "1px solid var(--border-2)",
                        borderRadius: 7,
                        padding: "5px 8px",
                        fontFamily: "var(--font-ui)",
                        fontSize: 12.5,
                        color: "var(--text-1)",
                        outline: "none",
                      }}
                    />
                    <button
                      type="button"
                      aria-label={`Remover opção ${i + 1}`}
                      onClick={() => removeOpt(i)}
                      className="tt-iconbtn"
                      style={{ width: 22, height: 22, background: "transparent", color: "var(--text-3)", flex: "0 0 auto" }}
                    >
                      <Icon.x size={12} />
                    </button>
                  </div>
                ))}
                <button
                  type="button"
                  aria-label="Adicionar opção"
                  onClick={addOpt}
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: 7,
                    alignSelf: "flex-start",
                    padding: "5px 6px",
                    border: "none",
                    background: "transparent",
                    color: "var(--text-3)",
                    font: "inherit",
                    fontSize: 12,
                    cursor: "pointer",
                  }}
                >
                  <Icon.plus size={12} sw={2.2} />
                  Adicionar opção
                </button>
              </div>
            )}

            <div style={{ display: "flex", justifyContent: "flex-end", gap: 8, marginTop: 2 }}>
              <button
                type="button"
                className="tt-btn tt-btn--ghost"
                aria-label="Cancelar nova propriedade"
                onClick={reset}
                style={{ padding: "6px 11px", fontSize: 12.5 }}
              >
                Cancelar
              </button>
              <button
                type="button"
                className="tt-btn tt-btn--accent"
                aria-label="Criar propriedade"
                disabled={!canCreate}
                onClick={submit}
                style={{ padding: "6px 14px", fontSize: 12.5, opacity: canCreate ? 1 : 0.5 }}
              >
                Criar
              </button>
            </div>
          </div>
        </AnchoredPopover>
      )}
    </>
  );
}
