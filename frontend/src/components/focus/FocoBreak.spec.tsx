// FocoBreak.spec — fase 15 T-07 (CA-14/14.1/15): o modo descanso
// redesenhado. Rótulo longa×curta via break_kind; "pausa encerrada" quando
// zera; "ajustar" navega pros Ajustes; retomar/pular/parar chamam o store.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import FocoBreak from './FocoBreak';
import { initialTimerState, useTimerStore } from '../../lib/timerStore';
import type { BreakKind, TimerState } from '../../lib/api';

function pausa(remaining: number, breakKind: BreakKind, target = 300): TimerState {
  return {
    mode: 'break',
    entry: null,
    elapsed_seconds: target - remaining,
    target_seconds: target,
    remaining_seconds: remaining,
    resume: { entry_id: 'e1', mode: 'pomodoro_focus', target_seconds: 900 },
    break_kind: breakKind,
  };
}

function seedTimer(state: TimerState | null) {
  useTimerStore.setState({
    state,
    anchorMs: Date.now(),
    offsetMs: 0,
    tick: 0,
    firedKey: null,
    transitioning: false,
  });
}

const acoes = {
  resume: useTimerStore.getState().resume,
  skip: useTimerStore.getState().skip,
  stop: useTimerStore.getState().stop,
};

function renderBreak() {
  return render(
    <MemoryRouter initialEntries={['/foco']}>
      <Routes>
        <Route path="/foco" element={<FocoBreak />} />
        <Route path="/ajustes" element={<div>rota-ajustes</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  useTimerStore.setState({ ...initialTimerState, ...acoes });
});

// CA-14 — render do modo descanso: rótulo, dígitos gigantes, presets, controles.
it('renderiza rótulo, dígitos, presets e controles (CA-14)', () => {
  seedTimer(pausa(300, 'short'));
  renderBreak();
  expect(screen.getByText('Pausa Pomodoro')).toBeInTheDocument();
  expect(screen.getByTestId('focus-digits').textContent).toBe('05:00');
  expect(screen.getByText('5 min')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Retomar' })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /pular pausa/i })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'ajustar' })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Parar' })).toBeInTheDocument();
});

// CA-14.1 — pausa longa tem rótulo distinto.
it('pausa longa mostra "Pausa longa" (CA-14.1)', () => {
  seedTimer(pausa(900, 'long', 900));
  renderBreak();
  expect(screen.getByText('Pausa longa')).toBeInTheDocument();
  expect(screen.queryByText('Pausa Pomodoro')).toBeNull();
});

// CA-14.1 — pausa curta mantém o rótulo padrão.
it('pausa curta mantém "Pausa Pomodoro" (CA-14.1)', () => {
  seedTimer(pausa(120, 'short'));
  renderBreak();
  expect(screen.getByText('Pausa Pomodoro')).toBeInTheDocument();
  expect(screen.queryByText('Pausa longa')).toBeNull();
});

// CA-15 — pausa zerada vira "Pausa encerrada" e some o "pular pausa".
it('pausa zerada mostra "Pausa encerrada" sem pular (CA-15)', () => {
  seedTimer(pausa(0, 'short'));
  renderBreak();
  expect(screen.getByText('Pausa encerrada')).toBeInTheDocument();
  expect(screen.queryByRole('button', { name: /pular pausa/i })).toBeNull();
  expect(screen.getByRole('button', { name: 'Retomar' })).toBeInTheDocument();
});

// CA-14 — "ajustar" navega pros Ajustes.
it('"ajustar" navega pra /ajustes (CA-14)', () => {
  seedTimer(pausa(300, 'short'));
  renderBreak();
  fireEvent.click(screen.getByRole('button', { name: 'ajustar' }));
  expect(screen.getByText('rota-ajustes')).toBeInTheDocument();
});

// CA-14 — retomar/pular/parar chamam resume/skip/stop do store.
it('Retomar, pular e Parar chamam resume/skip/stop (CA-14)', () => {
  const resumeSpy = vi.fn();
  const skipSpy = vi.fn();
  const stopSpy = vi.fn();
  useTimerStore.setState({ resume: resumeSpy, skip: skipSpy, stop: stopSpy });
  seedTimer(pausa(300, 'short'));
  renderBreak();
  fireEvent.click(screen.getByRole('button', { name: 'Retomar' }));
  expect(resumeSpy).toHaveBeenCalledTimes(1);
  fireEvent.click(screen.getByRole('button', { name: /pular pausa/i }));
  expect(skipSpy).toHaveBeenCalledTimes(1);
  fireEvent.click(screen.getByRole('button', { name: 'Parar' }));
  expect(stopSpy).toHaveBeenCalledTimes(1);
});
