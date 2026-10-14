// NoteEditor (fase 26 T-09, CA-17/18/19/20/21/22): título grande + textarea de
// markdown SEMPRE editável (sem botão Editar — RF-15), debounce PATCH ≥600ms
// (D-09), chips de link no topo (clicar abre a entidade), menu slash via
// lib/slash. Viewer → readOnly sem slash (CA-22). Tokens via inline var(--…).
import { useEffect, useRef, useState } from 'react';
import type { Note, NoteLink } from '../../lib/api';
import { usePatchNote, useCreateLink } from '../../lib/queries/notes';
import {
  detectSlash,
  filterItems,
  applySlashBlock,
  applyBlock,
  linkMarker,
  type SlashItem,
} from '../../lib/slash';
import SlashMenu from './SlashMenu';
import { dataBr } from '../board/dateBr';
import { Icon } from '../icons';

const DEBOUNCE_MS = 600;

export default function NoteEditor({
  wsId,
  projectId,
  note,
  canWrite,
  focusTitle,
  onOpenLink,
}: {
  wsId: string;
  projectId: string;
  note: Note;
  canWrite: boolean;
  focusTitle?: boolean;
  onOpenLink?: (link: NoteLink) => void;
}) {
  const patchNote = usePatchNote(wsId, projectId);
  const createLink = useCreateLink(wsId, projectId, note.id);

  const [title, setTitle] = useState(note.title);
  const [body, setBody] = useState(note.body_md);
  const [slashQuery, setSlashQuery] = useState<string | null>(null);
  const [slashSel, setSlashSel] = useState(0);
  const [linkError, setLinkError] = useState(false);
  const bodyRef = useRef<HTMLTextAreaElement>(null);
  const titleRef = useRef<HTMLInputElement>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // último input agendado mas ainda não persistido (pra FLUSH no blur/unmount —
  // fechar o painel antes dos 600ms não pode perder a edição).
  const pendingRef = useRef<{ title?: string; body_md?: string } | null>(null);
  // patchNote.mutate é estável (useMutation), mas guardamos numa ref pra o
  // cleanup do unmount poder disparar sem reexecutar o effect.
  const mutateRef = useRef(patchNote.mutate);
  mutateRef.current = patchNote.mutate;

  // flush imediato do PATCH pendente (blur/unmount): persiste sem esperar o
  // debounce. Sem pendente → no-op.
  const flushPending = () => {
    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
      debounceRef.current = null;
    }
    if (pendingRef.current) {
      const input = pendingRef.current;
      pendingRef.current = null;
      mutateRef.current({ id: note.id, input });
    }
  };

  // debounce do PATCH: agenda 600ms após a última tecla. Acumula title+body no
  // pendente (o último agendamento ganha; campos distintos não se sobrescrevem).
  const schedulePatch = (input: { title?: string; body_md?: string }) => {
    pendingRef.current = { ...pendingRef.current, ...input };
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      const pend = pendingRef.current;
      pendingRef.current = null;
      debounceRef.current = null;
      if (pend) mutateRef.current({ id: note.id, input: pend });
    }, DEBOUNCE_MS);
  };
  // flush no unmount (fechar o painel persiste a última edição). note.id é estável
  // por key (NotePanel keya por note.id).
  useEffect(
    () => () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
      if (pendingRef.current) {
        mutateRef.current({ id: note.id, input: pendingRef.current });
        pendingRef.current = null;
      }
    },
    [note.id],
  );

  // corpo = CANVAS INFINITO (estilo Notion, igual ao popup de task — fase 31): a
  // textarea CRESCE com o conteúdo (sem scroll interno) e a PÁGINA inteira rola.
  // autoGrow zera a altura e usa o scrollHeight.
  const autoGrow = (el: HTMLTextAreaElement | null) => {
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = `${el.scrollHeight}px`;
  };
  useEffect(() => {
    autoGrow(bodyRef.current); // ajusta a altura ao conteúdo inicial
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // nota recém-criada via /note ou "New note": abre FOCADA no título e seleciona
  // o conteúdo, pro Diogo nomear direto (fase 31 — captura à-prova-de-fricção).
  useEffect(() => {
    if (focusTitle && canWrite) {
      const el = titleRef.current;
      if (el) {
        el.focus();
        el.select();
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const items: SlashItem[] = slashQuery !== null ? filterItems(slashQuery) : [];

  const onBodyChange = (value: string, caret: number) => {
    setBody(value);
    autoGrow(bodyRef.current);
    schedulePatch({ body_md: value });
    if (!canWrite) return;
    const hit = detectSlash(value, caret);
    if (hit) {
      setSlashQuery(hit.query);
      setSlashSel(0);
    } else {
      setSlashQuery(null);
    }
  };

  const closeSlash = () => setSlashQuery(null);

  const pickItem = (item: SlashItem) => {
    const el = bodyRef.current;
    const caret = el ? el.selectionStart : body.length;
    if (item.kind === 'block') {
      // To-do / Heading / Divider: só insere markdown (CA-20), sem API.
      const out = applySlashBlock(body, caret, item.cmd);
      applyAndFocus(out.value, out.caret);
      closeSlash();
      return;
    }
    // Task / Note: chama a API de link; ao sucesso insere o marcador + chip
    // (CA-21). Falha (ex.: nota solta + task — CA-10) sinaliza e NÃO insere.
    const kind = item.kind === 'link-task' ? 'task' : 'note';
    const defaultName = kind === 'task' ? 'New task' : 'New note';
    setLinkError(false);
    closeSlash();
    createLink.mutate(
      kind === 'task' ? { kind, name: defaultName } : { kind, title: defaultName },
      {
        onSuccess: () => {
          const out = applyBlock(body, caret, linkMarker(kind, defaultName));
          applyAndFocus(out.value, out.caret);
        },
        onError: () => setLinkError(true),
      },
    );
  };

  const applyAndFocus = (value: string, caret: number) => {
    setBody(value);
    schedulePatch({ body_md: value });
    requestAnimationFrame(() => {
      const el = bodyRef.current;
      if (el) {
        autoGrow(el);
        el.focus();
        el.setSelectionRange(caret, caret);
      }
    });
  };

  const onBodyKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (slashQuery === null || items.length === 0) return;
    if (e.key === 'Escape') {
      e.preventDefault();
      closeSlash();
    } else if (e.key === 'Enter') {
      e.preventDefault();
      pickItem(items[slashSel]);
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      setSlashSel((s) => Math.min(s + 1, items.length - 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setSlashSel((s) => Math.max(s - 1, 0));
    }
  };

  return (
    // página de nota (fase 31): mesmo acabamento do popup de task — respira como
    // PÁGINA (padding generoso), SEM caixa e SEM scroll interno (o CenterPanel
    // rola). O corpo é o herói; a linha meta sob o título some ao subir. CA-N5/N6.
    <div
      style={{
        boxSizing: 'border-box',
        display: 'flex',
        flexDirection: 'column',
        gap: 14,
        padding: '26px 36px 60px',
      }}
    >
      <input
        ref={titleRef}
        aria-label="Título da nota"
        data-testid="note-title"
        value={title}
        readOnly={!canWrite}
        onChange={(e) => {
          setTitle(e.target.value);
          schedulePatch({ title: e.target.value });
        }}
        onKeyDown={(e) => {
          // Enter no título pula pro corpo (estilo Notion) — não insere quebra
          // (input já é single-line) e move o cursor pro fim do texto existente.
          if (e.key === 'Enter') {
            e.preventDefault();
            const el = bodyRef.current;
            if (el) {
              el.focus();
              el.setSelectionRange(body.length, body.length);
            }
          }
        }}
        onBlur={flushPending}
        placeholder="Untitled"
        style={{
          background: 'transparent',
          border: 'none',
          outline: 'none',
          color: 'var(--text-1)',
          fontFamily: 'var(--font-ui)',
          fontSize: 30,
          fontWeight: 700,
          letterSpacing: '-.02em',
          width: '100%',
        }}
      />

      {/* linha meta QUIETA sob o título (decisão HITL "página pura"): .md + chips
          de link (CA-17, clicar abre a entidade) + data de atualização. Tudo em
          tom faint — a página inteira rola e esta linha some ao subir no corpo. */}
      <div
        data-testid="note-meta"
        style={{
          display: 'flex',
          flexWrap: 'wrap',
          alignItems: 'center',
          gap: 8,
          fontSize: 12.5,
          color: 'var(--text-3)',
        }}
      >
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 5 }}>
          <Icon.doc size={13} />
          <span className="mono">.md</span>
        </span>
        {note.links.map((l) => (
          <button
            key={l.id}
            type="button"
            className="tt-chip"
            data-testid={`note-chip-${l.ref_type}`}
            onClick={() => onOpenLink?.(l)}
            style={{ display: 'inline-flex', alignItems: 'center', gap: 5, cursor: 'pointer' }}
          >
            {l.ref_type === 'task' ? <Icon.board size={12} /> : <Icon.doc size={12} />} {l.label}
          </button>
        ))}
        <span style={{ marginLeft: 'auto' }}>Updated {dataBr(note.updated_at.slice(0, 10))}</span>
      </div>

      {linkError && (
        <div role="alert" className="faint" data-testid="note-link-error" style={{ fontSize: 12, color: 'var(--danger)' }}>
          Não foi possível criar o link.
        </div>
      )}

      <div style={{ position: 'relative', display: 'flex' }}>
        <textarea
          ref={bodyRef}
          aria-label="Corpo da nota"
          data-testid="note-body"
          value={body}
          readOnly={!canWrite}
          onChange={(e) => onBodyChange(e.target.value, e.target.selectionStart)}
          onKeyDown={onBodyKeyDown}
          onBlur={() => {
            closeSlash();
            flushPending();
          }}
          placeholder="Escreva em Markdown…  toque / para comandos (Task, Note, To-do, Heading…)"
          style={{
            flex: 1,
            width: '100%',
            minHeight: '52vh',
            background: 'transparent',
            border: 'none',
            outline: 'none',
            resize: 'none',
            overflow: 'hidden',
            color: 'var(--text-1)',
            fontFamily: 'var(--font-ui)',
            fontSize: 15.5,
            lineHeight: 1.7,
            padding: 0,
          }}
        />
        {canWrite && slashQuery !== null && (
          <SlashMenu items={items} selected={slashSel} onPick={pickItem} />
        )}
      </div>
    </div>
  );
}
