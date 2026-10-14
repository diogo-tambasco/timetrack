// TaskPanelProps (fase 24 T-12, CA-20/26): linhas de propriedade NATIVAS do popup
// — Status (chip 1-clique cicla), Priority (chip 1-clique cicla), Due (date),
// Time (tracked real + botão Start MOCK/desabilitado data-testid="task-start" — a
// fase 30 troca SÓ a ação), Notion (input↔preview — T-13). Gating: can_write=false
// → chips não-clicáveis, date read-only, Start disabled (CA-25). Custom props = 25.
import { useRef, useState } from 'react';
import type { BoardTask, TaskStatus } from '../../lib/api';
import { useWorkspaceMembers } from '../../lib/queries/workspace';
import { Icon } from '../icons';
import { dataBr } from './dateBr';
import DatePicker from './DatePicker';
import AnchoredPopover from './AnchoredPopover';
import { STATUS_ORDER, PRIORITY_ORDER } from './status';
import {
  STATUS_META,
  PRIORITY_LABEL,
  priorityColor,
} from './status';
import NotionField from './NotionPreview';

type PropPatch = Partial<{
  status: TaskStatus;
  priority: number;
  due_date: string | null;
  notion_url: string | null;
  assignee: string | null;
  requested_by: string | null;
}>;

export default function TaskPanelProps({
  wsId,
  task,
  canWrite,
  onPatch,
  autoOpenDue = false,
}: {
  wsId: string;
  task: BoardTask;
  canWrite: boolean;
  onPatch: (patch: PropPatch) => void;
  // abre o calendário de Due automaticamente (criação via /task nome — fase 31)
  autoOpenDue?: boolean;
}) {
  // fase 18 (migrado pro popup — CA-28.1): Responsável/Solicitante por membro.
  const members = useWorkspaceMembers(wsId).data ?? [];
  // fase 31: workspace solo (1 membro) NÃO mostra atribuição — poluiria o
  // planejamento individual (o mockup não tem essas linhas). Multiplayer reabre.
  const multiplayer = members.length > 1;
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <Row label="Status" icon={<Icon.board size={13} />}>
        <StatusSelect value={task.status} canWrite={canWrite} onChange={(s) => onPatch({ status: s })} />
      </Row>

      <Row label="Priority" icon={<Icon.chart size={13} />}>
        <PrioritySelect value={task.priority} canWrite={canWrite} onChange={(p) => onPatch({ priority: p })} />
      </Row>

      <Row label="Due" icon={<Icon.cal size={13} />}>
        <DueField
          value={task.due_date}
          canWrite={canWrite}
          autoOpen={autoOpenDue}
          onChange={(v) => onPatch({ due_date: v })}
        />
      </Row>

      {multiplayer && (
        <>
          <Row label="Responsável" icon={<Icon.users size={13} />}>
            <MemberSelect
              ariaLabel="Responsável"
              value={task.assignee}
              members={members}
              canWrite={canWrite}
              emptyLabel="— sem responsável"
              onChange={(v) => onPatch({ assignee: v })}
            />
          </Row>

          <Row label="Solicitante" icon={<Icon.cliente size={13} />}>
            <MemberSelect
              ariaLabel="Solicitante"
              value={task.requested_by}
              members={members}
              canWrite={canWrite}
              emptyLabel="— ninguém"
              onChange={(v) => onPatch({ requested_by: v })}
            />
          </Row>
        </>
      )}

      <Row label="Link" icon={<Icon.link size={13} />}>
        <NotionField task={task} canWrite={canWrite} onPatch={(notion_url) => onPatch({ notion_url })} />
      </Row>
    </div>
  );
}

function MemberSelect({
  ariaLabel,
  value,
  members,
  canWrite,
  emptyLabel,
  onChange,
}: {
  ariaLabel: string;
  value: string | null;
  members: { user_id: string; name: string }[];
  canWrite: boolean;
  emptyLabel: string;
  onChange: (v: string | null) => void;
}) {
  if (!canWrite) {
    const m = members.find((x) => x.user_id === value);
    return <span style={{ fontSize: 13 }}>{m ? m.name : '—'}</span>;
  }
  return (
    <select
      aria-label={ariaLabel}
      value={value ?? ''}
      onChange={(e) => onChange(e.target.value === '' ? null : e.target.value)}
      style={{
        background: 'transparent',
        border: 'none',
        color: 'var(--text-1)',
        font: 'inherit',
        fontSize: 13,
        cursor: 'pointer',
        maxWidth: '100%',
      }}
    >
      <option value="">{emptyLabel}</option>
      {members.map((m) => (
        <option key={m.user_id} value={m.user_id}>
          {m.name}
        </option>
      ))}
    </select>
  );
}

// DueField: a Due em DESTAQUE — chip clicável (accent quando vazio: "convida" a
// agendar; sólido com a data quando marcado). Abre o DatePicker num POPOVER
// (AnchoredPopover/portal — não clipa nem empurra conteúdo). autoOpen abre já
// aberto (criação via /task nome — "quando preciso fazer isso?").
function DueField({
  value,
  canWrite,
  autoOpen,
  onChange,
}: {
  value: string | null;
  canWrite: boolean;
  autoOpen?: boolean;
  onChange: (v: string | null) => void;
}) {
  const [open, setOpen] = useState(!!autoOpen && canWrite);
  const anchorRef = useRef<HTMLButtonElement>(null);
  // minimalista: SEM balão/pill. Texto inline — data sólida quando marcada,
  // "Agendar…" fraquinho quando vazia. Sublinha no hover (afinidade de link).
  return (
    <>
      <button
        ref={anchorRef}
        type="button"
        aria-label="Due date"
        disabled={!canWrite}
        onClick={() => canWrite && setOpen((o) => !o)}
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          gap: 0,
          fontSize: 13,
          fontWeight: value ? 540 : 500,
          padding: '2px 0',
          border: 'none',
          background: 'transparent',
          color: value ? 'var(--text-1)' : 'var(--text-3)',
          cursor: canWrite ? 'pointer' : 'default',
          font: 'inherit',
        }}
      >
        {value ? dataBr(value) : 'Agendar…'}
      </button>
      {open && canWrite && (
        <AnchoredPopover anchorRef={anchorRef} onClose={() => setOpen(false)}>
          <DatePicker
            value={value}
            onPick={(iso) => {
              onChange(iso);
              setOpen(false);
            }}
          />
        </AnchoredPopover>
      )}
    </>
  );
}

// StatusSelect: dropdown minimalista (chip do status atual → menu com os 4
// estados, dot + label). Popover via portal (não clipa). Substitui o "cicla no
// clique" — escolha direta é mais previsível pra planejar.
function StatusSelect({
  value,
  canWrite,
  onChange,
}: {
  value: TaskStatus;
  canWrite: boolean;
  onChange: (s: TaskStatus) => void;
}) {
  const [open, setOpen] = useState(false);
  const anchorRef = useRef<HTMLButtonElement>(null);
  const meta = STATUS_META[value];
  return (
    <>
      <button
        ref={anchorRef}
        type="button"
        aria-label="Status"
        aria-haspopup="listbox"
        disabled={!canWrite}
        onClick={() => canWrite && setOpen((o) => !o)}
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          gap: 7,
          fontSize: 13,
          fontWeight: 540,
          padding: '3px 4px 3px 2px',
          borderRadius: 7,
          border: 'none',
          background: 'transparent',
          color: meta.color,
          cursor: canWrite ? 'pointer' : 'default',
          font: 'inherit',
        }}
      >
        <span aria-hidden style={{ width: 8, height: 8, borderRadius: '50%', background: meta.color }} />
        {meta.label}
        {canWrite && <Icon.chevron size={13} style={{ color: 'var(--text-3)' }} />}
      </button>
      {open && canWrite && (
        <AnchoredPopover anchorRef={anchorRef} onClose={() => setOpen(false)}>
          <div
            role="listbox"
            style={{
              minWidth: 168,
              background: 'var(--surface-2)',
              border: '1px solid var(--border-2)',
              borderRadius: 9,
              boxShadow: 'var(--shadow)',
              padding: 5,
            }}
          >
            {STATUS_ORDER.map((s) => {
              const m = STATUS_META[s];
              return (
                <button
                  key={s}
                  type="button"
                  role="option"
                  aria-selected={s === value}
                  onClick={() => {
                    onChange(s);
                    setOpen(false);
                  }}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 9,
                    width: '100%',
                    padding: '7px 9px',
                    borderRadius: 6,
                    border: 'none',
                    background: s === value ? 'var(--surface-3)' : 'transparent',
                    color: 'var(--text-1)',
                    font: 'inherit',
                    fontSize: 13,
                    cursor: 'pointer',
                    textAlign: 'left',
                  }}
                >
                  <span aria-hidden style={{ width: 8, height: 8, borderRadius: '50%', background: m.color, flex: '0 0 auto' }} />
                  {m.label}
                </button>
              );
            })}
          </div>
        </AnchoredPopover>
      )}
    </>
  );
}

// PriorityDot: bolinha do nível — cheia na cor do nível; "No priority" (0) é um
// anel vazado (sinaliza "sem prioridade" sem peso visual).
function PriorityDot({ p }: { p: number }) {
  if (p === 0) {
    return <span aria-hidden style={{ width: 8, height: 8, borderRadius: '50%', border: '1.5px solid var(--text-3)', boxSizing: 'border-box', flex: '0 0 auto' }} />;
  }
  return <span aria-hidden style={{ width: 8, height: 8, borderRadius: '50%', background: priorityColor(p), flex: '0 0 auto' }} />;
}

// PrioritySelect: dropdown minimalista (chip do nível atual → menu com os 4
// níveis). Mesmo padrão do StatusSelect (popover via portal). Substitui o "cicla
// no clique" → escolha direta, prioridade que de fato funciona pra planejar.
function PrioritySelect({
  value,
  canWrite,
  onChange,
}: {
  value: number;
  canWrite: boolean;
  onChange: (p: number) => void;
}) {
  const [open, setOpen] = useState(false);
  const anchorRef = useRef<HTMLButtonElement>(null);
  return (
    <>
      <button
        ref={anchorRef}
        type="button"
        aria-label="Priority"
        aria-haspopup="listbox"
        disabled={!canWrite}
        onClick={() => canWrite && setOpen((o) => !o)}
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          gap: 8,
          fontSize: 13,
          fontWeight: 540,
          padding: '3px 4px 3px 2px',
          borderRadius: 7,
          border: 'none',
          background: 'transparent',
          color: priorityColor(value),
          cursor: canWrite ? 'pointer' : 'default',
          font: 'inherit',
        }}
      >
        <PriorityDot p={value} />
        {PRIORITY_LABEL[value]}
        {canWrite && <Icon.chevron size={13} style={{ color: 'var(--text-3)' }} />}
      </button>
      {open && canWrite && (
        <AnchoredPopover anchorRef={anchorRef} onClose={() => setOpen(false)}>
          <div
            role="listbox"
            style={{
              minWidth: 168,
              background: 'var(--surface-2)',
              border: '1px solid var(--border-2)',
              borderRadius: 9,
              boxShadow: 'var(--shadow)',
              padding: 5,
            }}
          >
            {PRIORITY_ORDER.map((p) => (
              <button
                key={p}
                type="button"
                role="option"
                aria-selected={p === value}
                onClick={() => {
                  onChange(p);
                  setOpen(false);
                }}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 9,
                  width: '100%',
                  padding: '7px 9px',
                  borderRadius: 6,
                  border: 'none',
                  background: p === value ? 'var(--surface-3)' : 'transparent',
                  color: 'var(--text-1)',
                  font: 'inherit',
                  fontSize: 13,
                  cursor: 'pointer',
                  textAlign: 'left',
                }}
              >
                <PriorityDot p={p} />
                {PRIORITY_LABEL[p]}
              </button>
            ))}
          </div>
        </AnchoredPopover>
      )}
    </>
  );
}

function Row({ label, icon, children }: { label: string; icon?: React.ReactNode; children: React.ReactNode }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 10, minHeight: 33, fontSize: 13, borderRadius: 7, padding: '0 6px' }}>
      <span
        style={{
          width: 150,
          flex: '0 0 auto',
          color: 'var(--text-3)',
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          fontSize: 12.5,
        }}
      >
        {icon}
        <span>{label}</span>
      </span>
      <div style={{ flex: 1, minWidth: 0, display: 'flex', alignItems: 'center', gap: 8 }}>{children}</div>
    </div>
  );
}
