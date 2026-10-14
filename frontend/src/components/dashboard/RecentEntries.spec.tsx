// RecentEntries.spec — fase 13 T-05 (CA-06/07/09/11/24.1): agrupa por dia,
// pula HOJE (é do TodayList reusado), botões continuar/editar presentes, vazio
// → mensagem. Datas relativas ao "hoje" real (via os MESMOS helpers do
// componente), determinístico sem fake timers (que travam o render assíncrono).
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import RecentEntries from './RecentEntries';
import { useAuthStore } from '../../lib/auth';
import { browserTZ } from '../../lib/format';
import { addDaysISO, localDayISO } from '../../lib/recent';
import * as entriesQ from '../../lib/queries/entries';
import * as projectsQ from '../../lib/queries/projects';
import type { Project, TimeEntry } from '../../lib/api';

vi.mock('../../lib/queries/entries', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/queries/entries')>();
  return { ...mod, useRecentEntries: vi.fn(), useContinueEntry: vi.fn() };
});
vi.mock('../../lib/queries/projects', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/queries/projects')>();
  return { ...mod, useProjects: vi.fn() };
});

const ws = 'w1';
const projetos: Project[] = [
  {
    id: 'p1', workspace_id: ws, client_id: null, name: 'Lauda', color: '#4f9dde',
    weekly_hours_goal: null, visibility: 'shared', owner_user_id: 'u1',
    archived_at: null, created_at: '2026-06-01T10:00:00Z',
  },
];

function entry(id: string, startedAt: string): TimeEntry {
  return {
    id, workspace_id: ws, project_id: 'p1', task_id: null, description: id,
    tags: [], started_at: startedAt, stopped_at: startedAt, duration_seconds: 3600,
    source: 'manual',
  };
}

function renderRecent() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
  return render(<RecentEntries />, { wrapper });
}

beforeEach(() => {
  vi.clearAllMocks();
  useAuthStore.setState({
    status: 'authenticated',
    me: {
      user: { id: 'u1', email: 'a@b.c', name: 'Ana', avatar_url: null },
      workspaces: [{ id: ws, name: 'WS', role: 'owner' }],
    },
  });
  vi.mocked(projectsQ.useProjects).mockReturnValue({ data: projetos } as never);
  vi.mocked(entriesQ.useContinueEntry).mockReturnValue({ mutate: vi.fn() } as never);
});

describe('RecentEntries', () => {
  it('agrupa por dia anterior, pula HOJE (CA-07/24.1) e oferece continuar/editar (CA-09)', () => {
    const today = localDayISO(new Date().toISOString(), browserTZ());
    const yest = addDaysISO(today, -1);
    const three = addDaysISO(today, -3);
    vi.mocked(entriesQ.useRecentEntries).mockReturnValue({
      data: {
        entries: [
          entry('hoje', `${today}T12:00:00`),
          entry('ontem', `${yest}T09:00:00`),
          entry('antes', `${three}T10:00:00`),
        ],
        total_seconds: 10800,
      },
    } as never);

    renderRecent();

    // hoje é do TodayList reusado → NÃO aparece aqui (CA-24.1)
    expect(screen.queryByText('hoje')).toBeNull();
    // ontem e o dia anterior aparecem, com cabeçalho "Ontem"
    expect(screen.getByText('ontem')).toBeInTheDocument();
    expect(screen.getByText('antes')).toBeInTheDocument();
    expect(screen.getByText('Ontem')).toBeInTheDocument();
    // ações por linha (CA-09)
    expect(screen.getAllByRole('button', { name: 'Editar' }).length).toBe(2);
    expect(screen.getAllByRole('button', { name: 'Continuar' }).length).toBe(2);
  });

  it('sem dias anteriores → mensagem vazia (CA-11)', () => {
    const today = localDayISO(new Date().toISOString(), browserTZ());
    // só uma entry de HOJE → depois de pular hoje, nada resta
    vi.mocked(entriesQ.useRecentEntries).mockReturnValue({
      data: { entries: [entry('hoje', `${today}T12:00:00`)], total_seconds: 3600 },
    } as never);

    renderRecent();
    expect(screen.getByText(/nada cronometrado nos últimos dias/i)).toBeInTheDocument();
  });
});
