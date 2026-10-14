import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import ProjectChip from './ProjectChip';

// T-17 (CA-30): chip com a cor do projeto e barrinha .tt-goal com largura
// min(100, hours/goal×100)% — fill na cor do projeto, trocando pra
// var(--accent) quando o progresso atinge 100%; sem meta, sem barrinha.
describe('ProjectChip', () => {
  it('9h/18h → barrinha em 50% com fill na cor do projeto', () => {
    const { container } = render(
      <ProjectChip name="App mobile" color="#4f9dde" hours={9} goal={18} />,
    );

    const fill = container.querySelector('.tt-goal > i') as HTMLElement;
    expect(fill).not.toBeNull();
    expect(fill.style.width).toBe('50%');
    expect(fill.style.background).toBe('rgb(79, 157, 222)'); // #4f9dde
  });

  it('20h/18h → barrinha trava em 100% com fill accent', () => {
    const { container } = render(
      <ProjectChip name="App mobile" color="#4f9dde" hours={20} goal={18} />,
    );

    const fill = container.querySelector('.tt-goal > i') as HTMLElement;
    expect(fill.style.width).toBe('100%');
    expect(fill.style.background).toBe('var(--accent)');
  });

  it('exatamente 100% já troca pra accent (regra do ProjRow)', () => {
    const { container } = render(
      <ProjectChip name="App mobile" color="#4f9dde" hours={18} goal={18} />,
    );

    const fill = container.querySelector('.tt-goal > i') as HTMLElement;
    expect(fill.style.width).toBe('100%');
    expect(fill.style.background).toBe('var(--accent)');
  });

  it('sem meta → chip com cor, mas SEM .tt-goal', () => {
    const { container, getByText } = render(
      <ProjectChip name="App mobile" color="#4f9dde" hours={9} goal={null} />,
    );

    expect(getByText('App mobile')).toBeInTheDocument();
    const dot = container.querySelector('.tt-dot') as HTMLElement;
    expect(dot.style.background).toBe('rgb(79, 157, 222)');
    expect(container.querySelector('.tt-goal')).toBeNull();
  });
});
