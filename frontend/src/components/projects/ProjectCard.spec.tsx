import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import ProjectCard from './ProjectCard';
import type { ProjectSummary } from '../../lib/api';

const base: ProjectSummary = {
  id: 'p1',
  name: 'App mobile',
  color: '#4f9dde',
  client_id: null,
  deadline: null,
  task_count: 6,
  done_count: 1,
  open_count: 5,
  tracked_seconds: 511200, // 142h
  due_today_count: 0,
};

function renderCard(summary: Partial<ProjectSummary> = {}, opts: { canWrite?: boolean; clientName?: string | null } = {}) {
  return render(
    <MemoryRouter>
      <ProjectCard
        summary={{ ...base, ...summary }}
        clientName={opts.clientName ?? null}
        canWrite={opts.canWrite ?? true}
        onEdit={vi.fn()}
        onArchive={vi.fn()}
      />
    </MemoryRouter>,
  );
}

// Fase 28 T-08 (CA-16): card VIVO — horas reais, pendentes/total, deadline, dot
// "hoje". Mata o `0h` hardcoded.
describe('ProjectCard vivo (T-08)', () => {
  it('mostra as horas REAIS (142h), não 0h', () => {
    renderCard();
    expect(screen.getByText('142h')).toBeInTheDocument();
    expect(screen.queryByText('0h')).not.toBeInTheDocument();
  });

  it('mostra "X pendentes · Y no total"', () => {
    renderCard();
    expect(screen.getByText(/5 pendentes · 6 no total/)).toBeInTheDocument();
  });

  it('1 pendente fica no singular', () => {
    renderCard({ open_count: 1, task_count: 3 });
    expect(screen.getByText(/1 pendente · 3 no total/)).toBeInTheDocument();
  });

  it('dot "hoje" presente quando due_today_count > 0', () => {
    renderCard({ due_today_count: 2 });
    expect(screen.getByTestId('project-today')).toBeInTheDocument();
  });

  it('sem dot "hoje" quando due_today_count = 0', () => {
    renderCard({ due_today_count: 0 });
    expect(screen.queryByTestId('project-today')).not.toBeInTheDocument();
  });

  it('mostra o chip de prazo quando há deadline', () => {
    renderCard({ deadline: '2025-09-30' });
    expect(screen.getByTestId('project-deadline')).toHaveTextContent('30/09');
  });

  it('sem chip de prazo quando deadline é null', () => {
    renderCard({ deadline: null });
    expect(screen.queryByTestId('project-deadline')).not.toBeInTheDocument();
  });

  it('o nome linka pro detalhe /projetos/:id', () => {
    renderCard();
    expect(screen.getByRole('link', { name: 'App mobile' })).toHaveAttribute('href', '/projetos/p1');
  });

  it('"Abrir" leva ao mesmo detalhe', () => {
    renderCard();
    expect(screen.getByRole('link', { name: /abrir/i })).toHaveAttribute('href', '/projetos/p1');
  });

  it('mostra o nome do cliente', () => {
    renderCard({}, { clientName: 'Acme' });
    expect(screen.getByText('Acme')).toBeInTheDocument();
  });

  it('viewer (canWrite=false): sem Editar/Arquivar, mantém Abrir (CA-31)', () => {
    renderCard({}, { canWrite: false });
    expect(screen.queryByRole('button', { name: /editar/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /arquivar/i })).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: /abrir/i })).toBeInTheDocument();
  });
});
