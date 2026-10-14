// NotionPreview.spec — fase 24 T-13 (CA-24/25/RNF-03): preview mostra o título do
// slug, abrir tem target=_blank, desvincular faz PATCH null; sem link → input que
// faz PATCH; mark "N" usa tokens (sem hex literal); gating.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import NotionField, { NotionMark, ReferenceLinks } from './NotionPreview';
import { parseLinks, joinLinks } from '../../lib/linkPreview';
import { mkTask } from './testFixtures';

beforeEach(() => vi.clearAllMocks());

describe('NotionField com link (CA-24, chip mínimo fase 31)', () => {
  it('mostra o título do slug + abrir nova aba (subtítulo só no card)', () => {
    const task = mkTask({ notion_url: 'https://www.notion.so/OKRs-Q3-8f3a1c9b' });
    render(<NotionField task={task} canWrite onPatch={vi.fn()} />);
    expect(screen.getByText('OKRs Q3')).toBeInTheDocument();
    const link = screen.getByRole('link', { name: /abrir no notion/i });
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('href', task.notion_url);
  });

  it('desvincular faz PATCH null', () => {
    const onPatch = vi.fn();
    render(<NotionField task={mkTask({ notion_url: 'https://www.notion.so/X-abc123' })} canWrite onPatch={onPatch} />);
    fireEvent.click(screen.getByLabelText('Desvincular Notion'));
    expect(onPatch).toHaveBeenCalledWith(null);
  });

  it('can_write=false esconde desvincular (preview-only — CA-25)', () => {
    render(<NotionField task={mkTask({ notion_url: 'https://www.notion.so/X-abc123' })} canWrite={false} onPatch={vi.fn()} />);
    expect(screen.queryByLabelText('Desvincular Notion')).not.toBeInTheDocument();
  });
});

describe('NotionField sem link (CA-24/25)', () => {
  it('input → Enter faz PATCH com o link', () => {
    const onPatch = vi.fn();
    render(<NotionField task={mkTask({ notion_url: null })} canWrite onPatch={onPatch} />);
    const input = screen.getByLabelText('Link do Notion');
    fireEvent.change(input, { target: { value: 'https://www.notion.so/Doc-aaa111' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(onPatch).toHaveBeenCalledWith('https://www.notion.so/Doc-aaa111');
  });

  it('can_write=false → preview-only, sem input', () => {
    render(<NotionField task={mkTask({ notion_url: null })} canWrite={false} onPatch={vi.fn()} />);
    expect(screen.queryByLabelText('Link do Notion')).not.toBeInTheDocument();
  });
});

describe('NotionField multi-link (fase 31)', () => {
  it('adiciona um 2º link → PATCH com os dois juntos (separados por \\n)', () => {
    const onPatch = vi.fn();
    render(
      <NotionField task={mkTask({ notion_url: 'https://github.com/steven-tey/novel/issues/806' })} canWrite onPatch={onPatch} />,
    );
    fireEvent.click(screen.getByLabelText('Adicionar link')); // abre o input à direita
    const input = screen.getByLabelText('Link do Notion');
    fireEvent.change(input, { target: { value: 'https://www.notion.so/Doc-aaa111' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(onPatch).toHaveBeenCalledWith(
      'https://github.com/steven-tey/novel/issues/806\nhttps://www.notion.so/Doc-aaa111',
    );
  });

  it('dois links → dois chips; remover um faz PATCH só com o outro', () => {
    const onPatch = vi.fn();
    const url = 'https://github.com/steven-tey/novel/issues/806\nhttps://www.notion.so/Doc-aaa111';
    render(<NotionField task={mkTask({ notion_url: url })} canWrite onPatch={onPatch} />);
    expect(screen.getAllByTestId('notion-preview')).toHaveLength(2);
    fireEvent.click(screen.getByLabelText('Desvincular GitHub'));
    expect(onPatch).toHaveBeenCalledWith('https://www.notion.so/Doc-aaa111');
  });

  it('parseLinks / joinLinks: ida e volta', () => {
    expect(parseLinks('a\nb\n\n c ')).toEqual(['a', 'b', 'c']);
    expect(parseLinks(null)).toEqual([]);
    expect(joinLinks(['a', '', ' b '])).toBe('a\nb');
    expect(joinLinks([])).toBeNull();
  });
});

describe('ReferenceLinks (fase 31) — disclosure "Links" fechado por padrão', () => {
  it('fechado por padrão: cards ocultos; abrir mostra título + subtítulo', () => {
    render(<ReferenceLinks urls={['https://www.notion.so/OKRs-Q3-8f3a1c9b']} />);
    expect(screen.queryByText('OKRs Q3')).not.toBeInTheDocument(); // fechado
    fireEvent.click(screen.getByRole('button', { name: /links/i }));
    expect(screen.getByText('OKRs Q3')).toBeInTheDocument();
    expect(screen.getByText('notion.so')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /abrir no notion/i })).toHaveAttribute('target', '_blank');
  });

  it('múltiplos links (Notion + GitHub) aparecem juntos ao abrir', () => {
    render(
      <ReferenceLinks
        urls={['https://www.notion.so/OKRs-Q3-8f3a1c9b', 'https://github.com/steven-tey/novel/issues/806']}
      />,
    );
    fireEvent.click(screen.getByRole('button', { name: /links/i }));
    expect(screen.getByText('OKRs Q3')).toBeInTheDocument();
    expect(screen.getByText('Issue #806')).toBeInTheDocument();
    expect(screen.getByText('steven-tey/novel')).toBeInTheDocument();
    expect(screen.getAllByTestId('link-embed')).toHaveLength(2);
  });

  it('sem links: não renderiza nada', () => {
    const { container } = render(<ReferenceLinks urls={[]} />);
    expect(container).toBeEmptyDOMElement();
  });
});

describe('NotionMark usa tokens (RNF-03)', () => {
  it('o mark "N" usa var(--notion-mark-bg)/ink — ZERO hex literal', () => {
    render(<NotionMark />);
    const mark = screen.getByTestId('notion-mark');
    expect(mark.style.background).toContain('var(--notion-mark-bg)');
    expect(mark.style.color).toContain('var(--notion-mark-ink)');
    expect(mark.textContent).toBe('N');
  });
});
