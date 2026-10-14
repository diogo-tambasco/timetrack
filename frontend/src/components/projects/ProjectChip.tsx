// ProjectChip (T-17, CA-30 — DESIGN.md §3.1 "chip com cor; meta com
// barrinha"): .tt-chip + .tt-dot na cor do projeto e .tt-goal com largura
// min(100, hours/goal×100)%, fill na cor do projeto trocando pra
// var(--accent) ao atingir 100% (regra do ProjRow). `hours` vem por prop —
// as páginas passam 0 até a fase 03 ligar as horas reais. Sem meta, a
// barrinha não é renderizada.

// GoalBar é a barrinha isolada — ProjectCard a usa direto (o cartão já
// exibe o nome grande; o chip completo é pra contextos compactos).
export function GoalBar({
  color,
  hours,
  goal,
}: {
  color: string;
  hours: number;
  goal: number | null;
}) {
  if (goal === null || goal <= 0) return null;
  const pct = Math.min(100, Math.round((hours / goal) * 100));
  return (
    <div className="tt-goal">
      <i style={{ width: `${pct}%`, background: pct >= 100 ? 'var(--accent)' : color }} />
    </div>
  );
}

export default function ProjectChip({
  name,
  color,
  hours = 0,
  goal = null,
}: {
  name: string;
  color: string;
  hours?: number;
  goal?: number | null;
}) {
  return (
    <span
      style={{
        display: 'inline-flex',
        flexDirection: 'column',
        gap: 4,
        minWidth: 0,
        verticalAlign: 'top',
      }}
    >
      <span className="tt-chip" style={{ background: 'var(--surface-2)' }}>
        <i className="tt-dot" style={{ width: 7, height: 7, background: color }} />
        {name}
      </span>
      <GoalBar color={color} hours={hours} goal={goal} />
    </span>
  );
}
