// ProjectTabs (fase 24 T-10, CA-15): barra de abas do workspace de projeto —
// Overview · Tasks · Notes · Goals · Calendar. É a FONTE DE INTEGRAÇÃO das fases
// seguintes (D-13): 26 (Notes), 28 (Calendar/Overview rica), 29 (Goals)
// SUBSTITUEM os placeholders SEM tocar nesta barra nem no shell (ProjectDetail).
import { Icon } from '../icons';

export type ProjectTab = 'overview' | 'tasks' | 'notes' | 'goals' | 'calendar';

// ordem e rótulos fixos do shell — as fases seguintes só preenchem os corpos.
export const PROJECT_TABS: { key: ProjectTab; label: string; Glyph: typeof Icon.list }[] = [
  { key: 'overview', label: 'Overview', Glyph: Icon.chart },
  { key: 'tasks', label: 'Tasks', Glyph: Icon.board },
  { key: 'notes', label: 'Notes', Glyph: Icon.list },
  { key: 'goals', label: 'Goals', Glyph: Icon.check },
  { key: 'calendar', label: 'Calendar', Glyph: Icon.cal },
];

export default function ProjectTabs({
  active,
  onChange,
}: {
  active: ProjectTab;
  onChange: (t: ProjectTab) => void;
}) {
  return (
    <div
      role="tablist"
      aria-label="Abas do projeto"
      style={{
        display: 'flex',
        gap: 2,
        borderBottom: '1px solid var(--border)',
        overflowX: 'auto',
      }}
    >
      {PROJECT_TABS.map(({ key, label, Glyph }) => {
        const on = active === key;
        return (
          <button
            key={key}
            type="button"
            role="tab"
            aria-selected={on}
            onClick={() => onChange(key)}
            style={{
              border: 'none',
              background: 'transparent',
              cursor: 'pointer',
              fontFamily: 'var(--font-ui)',
              fontSize: 13.5,
              fontWeight: 540,
              padding: '10px 14px',
              display: 'flex',
              alignItems: 'center',
              gap: 7,
              whiteSpace: 'nowrap',
              color: on ? 'var(--text-1)' : 'var(--text-2)',
              borderBottom: on ? '2px solid var(--accent)' : '2px solid transparent',
              marginBottom: -1,
            }}
          >
            <Glyph size={15} />
            {label}
          </button>
        );
      })}
    </div>
  );
}
