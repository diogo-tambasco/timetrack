// CustomProperties (fase 25 T-10/T-11, CA-21): o bloco de propriedades CUSTOM no
// popup central — uma PropertyRow por def do projeto (herança do schema na UI — CA-17)
// + o botão "Add property". Vive no TaskPanel (NÃO em TaskPanelProps — separação de
// arquivo p/ não colidir com a fase 30, que é dona do bloco Time/Start). Omite o
// bloco inteiro se não há defs e o usuário não pode escrever (nada a mostrar/criar).
import type { BoardTask, PropertyDef } from "../../lib/api";
import PropertyRow from "./PropertyRow";
import AddPropertyButton from "./AddPropertyButton";

export default function CustomProperties({
  wsId,
  projectId,
  task,
  defs,
  canWrite,
}: {
  wsId: string;
  projectId: string;
  task: BoardTask;
  defs: PropertyDef[];
  canWrite: boolean;
}) {
  if (defs.length === 0 && !canWrite) return null;
  const ordered = [...defs].sort((a, b) => a.position - b.position);
  return (
    <div data-testid="custom-properties" style={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {ordered.map((def) => (
        <PropertyRow
          key={def.id}
          wsId={wsId}
          projectId={projectId}
          def={def}
          task={task}
          canWrite={canWrite}
        />
      ))}
      <AddPropertyButton wsId={wsId} projectId={projectId} canWrite={canWrite} />
    </div>
  );
}
