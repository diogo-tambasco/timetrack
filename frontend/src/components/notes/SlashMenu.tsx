// SlashMenu (fase 26 T-09, CA-19): menu Task·Note·To-do·Heading 1·Heading 2·
// Divider filtrado pela query após a /. ↑/↓ navega, Enter confirma, Esc/clique-
// fora fecha (a navegação por teclado fica no NoteEditor, que é dono do textarea
// e do foco). Tokens via inline var(--…) — sem hex literal (check-cores).
import type { SlashCmd, SlashItem } from '../../lib/slash';
import { Icon } from '../icons';

// ícone por comando (fiel ao mockup: task→board, note→doc, todo→check, headings/
// divider→list). CA-N2.
const SLASH_ICON: Record<SlashCmd, (typeof Icon)[keyof typeof Icon]> = {
  task: Icon.board,
  note: Icon.doc,
  todo: Icon.check,
  h1: Icon.list,
  h2: Icon.list,
  divider: Icon.list,
};

export default function SlashMenu({
  items,
  selected,
  onPick,
}: {
  items: SlashItem[];
  selected: number;
  onPick: (item: SlashItem) => void;
}) {
  if (items.length === 0) return null;
  return (
    <div
      data-testid="slash-menu"
      role="listbox"
      style={{
        position: 'absolute',
        zIndex: 5,
        marginTop: 4,
        width: 290,
        background: 'var(--surface-2)',
        border: '1px solid var(--border-2)',
        borderRadius: 10,
        boxShadow: 'var(--shadow)',
        padding: 6,
        maxHeight: 300,
        overflowY: 'auto',
      }}
    >
      {items.map((s, i) => {
        const Glyph = SLASH_ICON[s.cmd];
        return (
          <div
            key={s.cmd}
            role="option"
            aria-selected={i === selected}
            data-testid={`slash-item-${s.cmd}`}
            // mousedown (não click) pra disparar ANTES do blur do textarea, que
            // fecharia o menu antes do click registrar.
            onMouseDown={(e) => {
              e.preventDefault();
              onPick(s);
            }}
            style={{
              display: 'flex',
              gap: 10,
              alignItems: 'center',
              padding: '8px 10px',
              borderRadius: 7,
              cursor: 'pointer',
              background: i === selected ? 'var(--surface-3)' : 'transparent',
            }}
          >
            <Glyph size={15} style={{ color: 'var(--text-3)', flex: '0 0 auto' }} />
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontSize: 13, fontWeight: 540, color: 'var(--text-1)' }}>{s.label}</div>
              <div className="faint" style={{ fontSize: 11 }}>
                {s.desc}
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}
