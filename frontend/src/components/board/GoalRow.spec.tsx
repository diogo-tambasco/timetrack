// GoalRow.spec.tsx — fase 29 T-08 (CA-12/12.1/15): a linha da tabela Notion de
// Goals. Barra por MODO (goalProgress), coluna Horas com/sem meta, chip Done/Open,
// gating de escrita (can_write), à prova de culpa (sem cor de erro).
import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import GoalRow from './GoalRow';
import { mkObjective } from './testFixtures';
import type { BoardObjective } from '../../lib/api';

function renderRow(objective: BoardObjective, canWrite = true) {
  const onEdit = vi.fn();
  const onDelete = vi.fn();
  const onToggleStatus = vi.fn();
  render(
    <table>
      <tbody>
        <GoalRow
          objective={objective}
          color="#4f9dde"
          canWrite={canWrite}
          onEdit={onEdit}
          onDelete={onDelete}
          onToggleStatus={onToggleStatus}
        />
      </tbody>
    </table>,
  );
  return { onEdit, onDelete, onToggleStatus };
}

function barFill() {
  return screen.getByTestId('goal-bar-fill');
}

describe('GoalRow (CA-12/12.1/15)', () => {
  it('mostra nome, tasks done/total e a barra', () => {
    renderRow(mkObjective({ name: 'Roadmap Q3', task_count: 4, done_count: 2, done_pct: 50, progress_pct: 50 }));
    expect(screen.getByText('Roadmap Q3')).toBeInTheDocument();
    expect(screen.getByText('2/4')).toBeInTheDocument();
    expect(barFill().style.width).toBe('50%');
  });

  it('by_hours: a barra usa progress_pct (horas), não done_pct', () => {
    // 0 tasks done mas horas batendo a meta → barra cheia (100%), não 0%.
    renderRow(
      mkObjective({
        completion_mode: 'by_hours',
        goal_hours: 4,
        task_count: 3,
        done_count: 0,
        done_pct: 0,
        tracked_seconds: 14400,
        progress_pct: 100,
        status: 'done',
      }),
    );
    expect(barFill().style.width).toBe('100%');
    expect(barFill().style.background).toContain('accent');
  });

  it('coluna Horas: com meta mostra tracked/meta; sem meta só tracked', () => {
    const { onEdit } = renderRow(
      mkObjective({ completion_mode: 'by_hours', goal_hours: 4, tracked_seconds: 7200, progress_pct: 50 }),
    );
    expect(screen.getByText('2h00/4h')).toBeInTheDocument();
    expect(onEdit).not.toHaveBeenCalled();
  });

  it('sem meta de horas: coluna Horas mostra só o tracked (sem /Xh)', () => {
    renderRow(mkObjective({ goal_hours: null, tracked_seconds: 7200 }));
    // exatamente "2h00" — sem o sufixo "/Xh" (a coluna Tasks "0/0" tem barra mas
    // não é o texto de horas; checamos que não há horas-com-barra).
    expect(screen.getByText('2h00')).toBeInTheDocument();
    expect(screen.queryByText(/2h00\//)).not.toBeInTheDocument();
  });

  it('status done → chip Done; status open → chip Open', () => {
    const { unmount } = render(
      <table><tbody>
        <GoalRow objective={mkObjective({ status: 'done', done_pct: 100, progress_pct: 100 })} color="#4f9dde" canWrite onEdit={vi.fn()} onDelete={vi.fn()} onToggleStatus={vi.fn()} />
      </tbody></table>,
    );
    expect(screen.getByText('Done')).toBeInTheDocument();
    expect(screen.queryByText('Open')).not.toBeInTheDocument();
    unmount();
    renderRow(mkObjective({ status: 'open' }));
    expect(screen.getByText('Open')).toBeInTheDocument();
  });

  it('à prova de culpa: by_date vencido aberto NÃO tem cor/classe de erro (RNF-03)', () => {
    const { container } = render(
      <table><tbody>
        <GoalRow
          objective={mkObjective({ completion_mode: 'by_date', target_date: '2020-01-01', status: 'open', done_pct: 10, progress_pct: 10 })}
          color="#4f9dde" canWrite onEdit={vi.fn()} onDelete={vi.fn()} onToggleStatus={vi.fn()}
        />
      </tbody></table>,
    );
    expect(screen.getByText('Open')).toBeInTheDocument();
    expect(container.innerHTML).not.toContain('danger');
    expect(container.innerHTML).not.toContain('var(--danger)');
  });

  it('canWrite=false: esconde Editar/Excluir (CA-15)', () => {
    renderRow(mkObjective({}), false);
    expect(screen.queryByRole('button', { name: /editar objetivo/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /excluir objetivo/i })).not.toBeInTheDocument();
  });

  it('canWrite=true: Editar/Excluir chamam os callbacks', () => {
    const { onEdit, onDelete } = renderRow(mkObjective({}), true);
    screen.getByRole('button', { name: /editar objetivo/i }).click();
    screen.getByRole('button', { name: /excluir objetivo/i }).click();
    expect(onEdit).toHaveBeenCalled();
    expect(onDelete).toHaveBeenCalled();
  });

  it('CA-08: clicar no chip conclui/reabre (chip vira botão quando canWrite)', () => {
    const { onToggleStatus } = renderRow(mkObjective({ status: 'open' }), true);
    screen.getByRole('button', { name: /concluir goal/i }).click();
    expect(onToggleStatus).toHaveBeenCalled();
  });

  it('canWrite=false: o chip de status NÃO é botão (só leitura)', () => {
    renderRow(mkObjective({ status: 'open' }), false);
    expect(screen.queryByRole('button', { name: /concluir goal/i })).not.toBeInTheDocument();
    expect(screen.getByText('Open')).toBeInTheDocument();
  });

  it('barra ≥100% usa accent; abaixo usa a cor do projeto', () => {
    const { unmount } = render(
      <table><tbody>
        <GoalRow objective={mkObjective({ done_pct: 100, progress_pct: 100 })} color="#4f9dde" canWrite onEdit={vi.fn()} onDelete={vi.fn()} onToggleStatus={vi.fn()} />
      </tbody></table>,
    );
    expect(within(screen.getByTestId('goal-row')).getByTestId('goal-bar-fill').style.background).toContain('accent');
    unmount();
    renderRow(mkObjective({ done_pct: 40, progress_pct: 40 }));
    // abaixo de 100% usa a cor do projeto (jsdom normaliza #4f9dde → rgb): só
    // garantimos que NÃO é o accent (token), provando que é a cor passada.
    expect(barFill().style.background).not.toContain('accent');
    expect(barFill().style.background).toContain('rgb(79, 157, 222)');
  });
});
