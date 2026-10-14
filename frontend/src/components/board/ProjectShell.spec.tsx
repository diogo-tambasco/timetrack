// ProjectShell.spec.tsx — fase 24 T-10 (CA-15): o tab-shell do projeto —
// ProjectTabs + ProjectOverview. Renderiza as 5 abas, clicar troca a aba, a
// Overview mostra KPIs + andamento por status, e Notes/Goals/Calendar mostram
// placeholder. (O ProjectDetail real consome useBoard — aqui testamos as peças
// puras com board mockado.)
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { useState } from 'react';
import ProjectTabs, { PROJECT_TABS, type ProjectTab } from './ProjectTabs';
import ProjectOverview from './ProjectOverview';
import { mkBoard } from './testFixtures';

function Shell() {
  const [tab, setTab] = useState<ProjectTab>('overview');
  return (
    <>
      <ProjectTabs active={tab} onChange={setTab} />
      {tab === 'overview' && <ProjectOverview board={mkBoard()} />}
      {tab === 'notes' && <div data-testid="ph-notes">Notes — em breve.</div>}
    </>
  );
}

describe('ProjectTabs (CA-15)', () => {
  it('renderiza as 5 abas Overview/Tasks/Notes/Goals/Calendar', () => {
    render(<ProjectTabs active="tasks" onChange={vi.fn()} />);
    for (const { label } of PROJECT_TABS) {
      expect(screen.getByRole('tab', { name: new RegExp(label, 'i') })).toBeInTheDocument();
    }
    expect(screen.getByRole('tab', { name: /tasks/i })).toHaveAttribute('aria-selected', 'true');
  });

  it('clicar numa aba chama onChange', () => {
    const onChange = vi.fn();
    render(<ProjectTabs active="overview" onChange={onChange} />);
    fireEvent.click(screen.getByRole('tab', { name: /goals/i }));
    expect(onChange).toHaveBeenCalledWith('goals');
  });

  it('trocar de aba renderiza o corpo certo (placeholder de Notes)', () => {
    render(<Shell />);
    expect(screen.queryByTestId('ph-notes')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('tab', { name: /notes/i }));
    expect(screen.getByTestId('ph-notes')).toBeInTheDocument();
  });
});

describe('ProjectOverview (CA-15)', () => {
  it('mostra KPIs e o andamento por status do board (sem inventar dados)', () => {
    render(<ProjectOverview board={mkBoard()} />);
    expect(screen.getByText('Tempo total')).toBeInTheDocument();
    expect(screen.getByText('Esta semana')).toBeInTheDocument();
    // andamento por status — os 4 rótulos do mockup
    expect(screen.getByText('Backlog')).toBeInTheDocument();
    expect(screen.getByText('To do')).toBeInTheDocument();
    expect(screen.getByText('In progress')).toBeInTheDocument();
    expect(screen.getByText('Done')).toBeInTheDocument();
  });
});
