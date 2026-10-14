// Spotlight.spec.tsx — T-05 (CA-01/04/05/06/07/08/16.1 + 27-H2/H3): conteúdo no
// CenterPanel, teclado, foco, filtro, contexto pelo PATHNAME (matchPath, não
// useParams) e gate do useBoard.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, waitFor, act } from '@testing-library/react';
import { MemoryRouter, Routes, Route, Outlet } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import Spotlight from './Spotlight';
import { useSpotlightStore } from '../../lib/spotlightStore';
import * as actions from '../../lib/spotlightActions';
import * as api from '../../lib/api';

// useCurrentWorkspace → ws fixo
vi.mock('../../lib/auth', () => ({
  useCurrentWorkspace: () => ({ id: 'w1', name: 'WS', role: 'owner' }),
}));

// runSpot espionado pra afirmar o ctx recebido
const runSpotSpy = vi.spyOn(actions, 'runSpot').mockResolvedValue(undefined);

// getBoard espionado pelo gate (CA-16.1)
const getBoardSpy = vi
  .spyOn(api, 'getBoard')
  .mockResolvedValue({ project: { id: 'p1', can_write: true } } as never);

function renderAt(path: string) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  // o Spotlight num elemento de LAYOUT (acima do segmento :id), espelhando o
  // AppShell: prova que o contexto vem do pathname, não de useParams (27-H2).
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route
            element={
              <>
                <Spotlight />
                <Outlet />
              </>
            }
          >
            <Route path="/projetos/:id" element={<div>projeto</div>} />
            <Route path="/relatorios" element={<div>relatorios</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  useSpotlightStore.setState({ open: false, query: '' });
  runSpotSpy.mockClear();
  getBoardSpy.mockClear();
});
afterEach(cleanup);

describe('Spotlight', () => {
  it('open=false ⇒ nada renderizado (CA-05)', () => {
    renderAt('/relatorios');
    expect(screen.queryByTestId('spotlight')).toBeNull();
  });

  it('aberto ⇒ input focado (CA-01)', async () => {
    renderAt('/relatorios');
    act(() => useSpotlightStore.getState().openSpotlight());
    await waitFor(() => {
      expect(document.activeElement).toBe(screen.getByTestId('spotlight-input'));
    });
  });

  it('digitar /ta ⇒ só o item /task (CA-07)', () => {
    renderAt('/relatorios');
    act(() => useSpotlightStore.getState().openSpotlight());
    fireEvent.change(screen.getByTestId('spotlight-input'), { target: { value: '/ta' } });
    expect(screen.getByTestId('spot-item-task')).toBeTruthy();
    expect(screen.queryByTestId('spot-item-note')).toBeNull();
    expect(screen.queryByTestId('spot-item-timer')).toBeNull();
  });

  it('título realçado aparece p/ /task X (CA-08)', () => {
    renderAt('/projetos/p1');
    act(() => useSpotlightStore.getState().openSpotlight('/task Comprar domínio'));
    expect(screen.getByText('Comprar domínio')).toBeTruthy();
  });

  it('Esc fecha (CA-04)', () => {
    renderAt('/relatorios');
    act(() => useSpotlightStore.getState().openSpotlight());
    fireEvent.keyDown(screen.getByTestId('spotlight-input'), { key: 'Escape' });
    expect(useSpotlightStore.getState().open).toBe(false);
  });

  it('clique no scrim fecha (CA-04)', () => {
    renderAt('/relatorios');
    act(() => useSpotlightStore.getState().openSpotlight());
    fireEvent.click(screen.getByTestId('center-scrim'));
    expect(useSpotlightStore.getState().open).toBe(false);
  });

  it('Enter chama runSpot com a query e o ctx (CA-09 via runSpot)', () => {
    renderAt('/relatorios');
    act(() => useSpotlightStore.getState().openSpotlight('/timer Estudar'));
    fireEvent.keyDown(screen.getByTestId('spotlight-input'), { key: 'Enter' });
    expect(runSpotSpy).toHaveBeenCalledTimes(1);
    const [input, ctx] = runSpotSpy.mock.calls[0];
    expect(input).toBe('/timer Estudar');
    expect(ctx.ws).toBe('w1');
  });

  // 27-FIND-01 — barra-sem-comando-válido ("/x", "/ta" parcial): menu vazio/parcial
  // (CA-07) ⇒ Enter é NO-OP (não cai no runSpot, que iniciaria um timer "/x").
  it('barra-sem-comando-válido (/x) + Enter ⇒ NÃO chama runSpot (à-prova-de-culpa)', () => {
    renderAt('/relatorios');
    act(() => useSpotlightStore.getState().openSpotlight('/x'));
    fireEvent.keyDown(screen.getByTestId('spotlight-input'), { key: 'Enter' });
    expect(runSpotSpy).not.toHaveBeenCalled();
  });

  it('comando PARCIAL (/ta) + Enter ⇒ NÃO chama runSpot (o usuário completa /task)', () => {
    renderAt('/relatorios');
    act(() => useSpotlightStore.getState().openSpotlight('/ta'));
    fireEvent.keyDown(screen.getByTestId('spotlight-input'), { key: 'Enter' });
    expect(runSpotSpy).not.toHaveBeenCalled();
  });

  // CONTROLE do no-op: texto livre (sem barra) ainda DISPARA o runSpot (RF-19).
  // Mata a mutação que tornaria o Enter no-op pra tudo.
  it('texto livre (sem barra) + Enter ⇒ CHAMA runSpot (RF-19)', () => {
    renderAt('/relatorios');
    act(() => useSpotlightStore.getState().openSpotlight('Estudar Go'));
    fireEvent.keyDown(screen.getByTestId('spotlight-input'), { key: 'Enter' });
    expect(runSpotSpy).toHaveBeenCalledTimes(1);
    expect(runSpotSpy.mock.calls[0][0]).toBe('Estudar Go');
  });

  it('↑/↓ trocam a seleção (CA-06)', () => {
    renderAt('/relatorios');
    act(() => useSpotlightStore.getState().openSpotlight()); // todos os 4 itens
    const inp = screen.getByTestId('spotlight-input');
    // 1º selecionado por padrão (task), seta pro 2º (note)
    expect(screen.getByTestId('spot-item-task').className).toContain('sel');
    fireEvent.keyDown(inp, { key: 'ArrowDown' });
    expect(screen.getByTestId('spot-item-note').className).toContain('sel');
    fireEvent.keyDown(inp, { key: 'ArrowUp' });
    expect(screen.getByTestId('spot-item-task').className).toContain('sel');
  });

  // 27-H2 — montado FORA da rota :id (em layout), o ctx.projectId vem do PATHNAME.
  it('projeto de contexto resolvido do pathname (matchPath, não useParams) — 27-H2', () => {
    renderAt('/projetos/abc-123');
    act(() => useSpotlightStore.getState().openSpotlight('/timer'));
    fireEvent.keyDown(screen.getByTestId('spotlight-input'), { key: 'Enter' });
    const [, ctx] = runSpotSpy.mock.calls[0];
    expect(ctx.projectId).toBe('abc-123');
  });

  it('fora de /projetos/:id ⇒ ctx.projectId null', () => {
    renderAt('/relatorios');
    act(() => useSpotlightStore.getState().openSpotlight('/timer'));
    fireEvent.keyDown(screen.getByTestId('spotlight-input'), { key: 'Enter' });
    const [, ctx] = runSpotSpy.mock.calls[0];
    expect(ctx.projectId).toBeNull();
  });

  // 27-H3/CA-16.1 — fechado e/ou fora de /projetos/:id ⇒ NENHUM getBoard.
  it('fechado ⇒ nenhum getBoard (CA-16.1)', () => {
    renderAt('/relatorios');
    expect(getBoardSpy).not.toHaveBeenCalled(); // open=false
  });

  it('aberto FORA de /projetos/:id ⇒ nenhum getBoard (CA-16.1)', async () => {
    renderAt('/relatorios');
    act(() => useSpotlightStore.getState().openSpotlight());
    await waitFor(() => expect(screen.getByTestId('spotlight')).toBeTruthy());
    expect(getBoardSpy).not.toHaveBeenCalled();
  });

  it('aberto DENTRO de /projetos/:id ⇒ getBoard dispara 1× (gate ativo)', async () => {
    renderAt('/projetos/p1');
    act(() => useSpotlightStore.getState().openSpotlight());
    await waitFor(() => expect(getBoardSpy).toHaveBeenCalledWith('w1', 'p1'));
  });
});
