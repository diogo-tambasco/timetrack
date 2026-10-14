// NoticeBanner.spec — fase 16 T-10 (CA-12): o banner aparece quando um notice
// de timer_auto_cut chega, ✕ dispensa (sem chamada de API), kind desconhecido
// é ignorado, banner ausente sem notice.
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import NoticeBanner from './NoticeBanner';
import { useNoticeStore, pushNotice } from '../lib/noticeStore';

// emite um notice DENTRO de act() pra o React re-renderizar o banner no teste
// (o store é puro/fora-do-React; pushNotice é chamado pelo attachTimerSync em
// produção, onde a re-render acontece naturalmente).
function emit(n: Parameters<typeof pushNotice>[0]) {
  act(() => pushNotice(n));
}

beforeEach(() => {
  useNoticeStore.setState({ notice: null });
  vi.restoreAllMocks();
});

describe('NoticeBanner', () => {
  it('sem notice, o banner não renderiza', () => {
    render(<NoticeBanner />);
    expect(screen.queryByTestId('notice-banner')).toBeNull();
  });

  it('notice de timer_auto_cut mostra o banner de corte', () => {
    render(<NoticeBanner />);
    emit({ kind: 'timer_auto_cut', entry_id: 'e-1', stopped_at: '2026-06-20T10:00:00Z' });
    const banner = screen.getByTestId('notice-banner');
    expect(banner).toBeTruthy();
    expect(banner).toHaveTextContent(/pausado automaticamente após 2h/i);
    // dispensável: tem o ✕ acessível e role=status
    expect(banner).toHaveAttribute('role', 'status');
    expect(screen.getByRole('button', { name: /dispensar aviso/i })).toBeTruthy();
  });

  it('✕ dispensa o banner sem chamada de API', () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch');
    render(<NoticeBanner />);
    emit({ kind: 'timer_auto_cut' });
    expect(screen.getByTestId('notice-banner')).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: /dispensar aviso/i }));

    expect(screen.queryByTestId('notice-banner')).toBeNull();
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it('notice de task_assigned mostra o banner de nova task (fase 18, CA-17)', () => {
    render(<NoticeBanner />);
    emit({ kind: 'task_assigned', task_id: 't-1', by: 'u-2' });
    const banner = screen.getByTestId('notice-banner');
    expect(banner).toBeTruthy();
    expect(banner).toHaveTextContent(/recebeu uma nova task/i);
    // segue dispensável (não toca o ramo timer_auto_cut)
    expect(screen.getByRole('button', { name: /dispensar aviso/i })).toBeTruthy();
  });

  it('notice de reminder mostra o banner de lembrete com o título (fase 20, CA-27)', () => {
    render(<NoticeBanner />);
    emit({ kind: 'reminder', task_id: 't-9', title: 'Entregar relatório' });
    const banner = screen.getByTestId('notice-banner');
    expect(banner).toBeTruthy();
    expect(banner).toHaveTextContent(/Lembrete: Entregar relatório/i);
    // segue dispensável; NÃO toca os ramos timer_auto_cut/task_assigned
    expect(screen.getByRole('button', { name: /dispensar aviso/i })).toBeTruthy();
  });

  it('notice de kind desconhecido não mostra o banner', () => {
    render(<NoticeBanner />);
    emit({ kind: 'qualquer_outra_coisa' });
    expect(screen.queryByTestId('notice-banner')).toBeNull();
  });

  it('Esc dispensa o banner', () => {
    render(<NoticeBanner />);
    emit({ kind: 'timer_auto_cut' });
    const banner = screen.getByTestId('notice-banner');
    fireEvent.keyDown(banner, { key: 'Escape' });
    expect(screen.queryByTestId('notice-banner')).toBeNull();
  });
});
