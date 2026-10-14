import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import ViewToggle, { useView } from './ViewToggle';

// Harness igual ao uso real nas páginas: hook de persistência + control.
function Harness({ k }: { k: string }) {
  const [view, setView] = useView(k, ['cards', 'table'] as const, 'cards');
  return (
    <div>
      <ViewToggle view={view} onChange={setView} />
      <output data-testid="view-atual">{view}</output>
    </div>
  );
}

beforeEach(() => {
  localStorage.clear();
});

// T-14 (CA-27): segmented control Cartões ↔ Tabela com default cards e
// preferência persistida por chave no localStorage.
describe('ViewToggle', () => {
  it('default cards sem preferência salva', () => {
    render(<Harness k="tt.view.teste" />);

    expect(screen.getByTestId('view-atual')).toHaveTextContent('cards');
    expect(screen.getByRole('button', { name: /cartões/i })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: /tabela/i })).toHaveAttribute('aria-pressed', 'false');
  });

  it('clique alterna e persiste na chave dada', () => {
    render(<Harness k="tt.view.teste" />);

    fireEvent.click(screen.getByRole('button', { name: /tabela/i }));
    expect(screen.getByTestId('view-atual')).toHaveTextContent('table');
    expect(localStorage.getItem('tt.view.teste')).toBe('table');

    fireEvent.click(screen.getByRole('button', { name: /cartões/i }));
    expect(screen.getByTestId('view-atual')).toHaveTextContent('cards');
    expect(localStorage.getItem('tt.view.teste')).toBe('cards');
  });

  it('remonta lendo o localStorage', () => {
    localStorage.setItem('tt.view.teste', 'table');

    const { unmount } = render(<Harness k="tt.view.teste" />);
    expect(screen.getByTestId('view-atual')).toHaveTextContent('table');

    unmount();
    render(<Harness k="tt.view.teste" />);
    expect(screen.getByTestId('view-atual')).toHaveTextContent('table');
  });

  it('valor desconhecido no storage cai no default cards', () => {
    localStorage.setItem('tt.view.teste', 'banana');

    render(<Harness k="tt.view.teste" />);
    expect(screen.getByTestId('view-atual')).toHaveTextContent('cards');
  });
});
