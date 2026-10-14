// RemoteShortcuts.spec — fase 23 T-06 (CA-09/10): atalhos pra Minhas tasks (18)
// e Modo Foco (/foco da 05). Só links (sem badge — degrada CA-09), sem throw.
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import RemoteShortcuts from './RemoteShortcuts';

function renderShortcuts() {
  return render(
    <MemoryRouter>
      <RemoteShortcuts />
    </MemoryRouter>,
  );
}

describe('RemoteShortcuts', () => {
  it('linka "Modo Foco" pra /foco (CA-10)', () => {
    renderShortcuts();
    const foco = screen.getByRole('link', { name: /modo foco/i });
    expect(foco).toHaveAttribute('href', '/foco');
  });

  it('linka "Minhas tasks" pra /minhas-tarefas (CA-09)', () => {
    renderShortcuts();
    const tasks = screen.getByRole('link', { name: /minhas tasks/i });
    expect(tasks).toHaveAttribute('href', '/minhas-tarefas');
  });

  it('sem hook de contagem → sem badge, sem throw', () => {
    renderShortcuts();
    // a fase 18 não expõe contagem barata → o atalho é só link (degrada CA-09)
    expect(screen.getByTestId('remote-shortcuts')).toBeInTheDocument();
    expect(screen.getAllByRole('link')).toHaveLength(2);
  });
});
