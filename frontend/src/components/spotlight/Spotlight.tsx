// Spotlight.tsx (fase 27, CA-01/04/05/06/07/08/16.1): o conteúdo do Spotlight
// renderizado DENTRO do CenterPanel da fase 24 (decisão A — scrim/card vêm de
// lá; classes próprias só de conteúdo `.spotitem`, tokens). Input + lista de
// comandos + chips de tecla + navegação por teclado. open=false ⇒ null.
//
// Contexto: ws de useCurrentWorkspace; projeto de contexto = matchPath do
// pathname (27-H2/D-13 — NÃO useParams, o Spotlight está no AppShell pai);
// can_write via useBoard GATEADO (27-H3/CA-16.1 — nunca dispara /board fora de
// /projetos/:id nem fechado).
import { useEffect, useRef, useState } from 'react';
import { matchPath, useLocation, useNavigate } from 'react-router-dom';
import { useCurrentWorkspace } from '../../lib/auth';
import { useBoard } from '../../lib/queries/objectives';
import { useSpotlightStore } from '../../lib/spotlightStore';
import { useOpenTaskStore } from '../../lib/openTaskStore';
import { useOpenNoteStore } from '../../lib/openNoteStore';
import { spotItems, spotParse, spotTitle } from '../../lib/spotlight';
import { runSpot, type SpotCtx } from '../../lib/spotlightActions';
import CenterPanel from '../board/CenterPanel';
import { Icon } from '../icons';

// resolve o projeto de contexto do PATHNAME (27-H2): o Spotlight vive no shell
// pai, então useParams() devolveria null — usa matchPath como SideNav/MobileNav.
export function contextProjectId(pathname: string): string | null {
  return matchPath('/projetos/:id', pathname)?.params.id ?? null;
}

export default function Spotlight() {
  const open = useSpotlightStore((s) => s.open);
  if (!open) return null; // CA-05 — nada renderizado fechado
  return <SpotlightBody />;
}

function SpotlightBody() {
  const query = useSpotlightStore((s) => s.query);
  const setQuery = useSpotlightStore((s) => s.setQuery);
  const close = useSpotlightStore((s) => s.close);
  const ws = useCurrentWorkspace();
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const openTask = useOpenTaskStore((s) => s.open);
  const openNote = useOpenNoteStore((s) => s.open);
  const projectId = contextProjectId(pathname);

  const inputRef = useRef<HTMLInputElement>(null);
  const [sel, setSel] = useState(0);
  // can_write do projeto de contexto, elevado pelo BoardGate (27-H3/CA-16.1).
  // Default true: otimista — o backend nega viewer/arquivado de qualquer jeito
  // (CA-16 só evita a chamada que já SABEMOS que falharia). Sem projeto de
  // contexto, `runSpot` já barra task/note/goal no guard de projectId (CA-15).
  const [canWrite, setCanWrite] = useState(true);
  const items = spotItems(query);

  // CA-01 — foca o input ao abrir (o body só monta com open=true)
  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  // CA-07 — a seleção volta a 0 a cada mudança de query (filtro)
  const onQuery = (q: string) => {
    setQuery(q);
    setSel(0);
  };

  const ctx: SpotCtx = { ws: ws?.id ?? '', projectId, canWrite };

  const exec = () => {
    if (!ws) return;
    // barra-sem-comando-válido ("/x", "/ta" parcial): o menu está vazio/parcial
    // (CA-07) → NO-OP. NÃO cair no runSpot, que trataria "/x" como texto livre e
    // iniciaria/editaria o timer com a descrição "/x" — surpreendente. À-prova-de-
    // culpa: a barra sinaliza intenção de comando; sem comando EXATO, nada acontece.
    // Texto livre (sem "/") segue normalmente pro runSpot (RF-19 — start/editCurrent).
    if (query.startsWith('/') && spotParse(query).act === null) return;
    // openTask: /task cria a entidade e abre o popup COMPLETO (Notion-like) já
    // focado no título quando nasceu sem nome — não mais "task fantasma" + navega.
    void runSpot(query, ctx, { close, navigate, openTask, openNote });
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      exec();
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      setSel((s) => Math.min(s + 1, Math.max(items.length - 1, 0)));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setSel((s) => Math.max(s - 1, 0));
    } else if (e.key === 'Escape') {
      e.preventDefault();
      close();
    }
  };

  const title = spotTitle(query);
  const parsedAct = spotParse(query).act;

  return (
    <>
      {projectId && <BoardGate ws={ws?.id ?? ''} projectId={projectId} onCanWrite={setCanWrite} />}
      <CenterPanel ariaLabel="Captura rápida (Cmd+K)" onClose={close}>
        <div data-testid="spotlight" style={{ display: 'flex', flexDirection: 'column' }}>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 12,
              padding: '17px 20px',
              borderBottom: '1px solid var(--border)',
            }}
          >
            <Icon.bolt size={18} sw={1.7} style={{ color: 'var(--accent)' }} />
            <input
              ref={inputRef}
              id="spotInput"
              data-testid="spotlight-input"
              value={query}
              onChange={(e) => onQuery(e.target.value)}
              onKeyDown={onKeyDown}
              placeholder="Comando ou busca…   /task Comprar domínio   ·   /note   ·   /timer"
              style={{
                flex: 1,
                background: 'transparent',
                border: 'none',
                outline: 'none',
                color: 'var(--text-1)',
                font: 'inherit',
                fontSize: 17,
              }}
            />
            <span className="chip mono" style={{ fontSize: 10 }}>
              esc
            </span>
          </div>

          <div style={{ maxHeight: '56vh', overflowY: 'auto', padding: 8 }}>
            {items.length === 0 ? (
              <div className="faint" style={{ padding: 16, fontSize: 13 }}>
                Nenhum comando
              </div>
            ) : (
              items.map((s, i) => {
                const Glyph = Icon[s.icon as keyof typeof Icon] ?? Icon.bolt;
                const showTitle = i === sel && title && (s.act === 'task' || s.act === 'note');
                return (
                  <div
                    key={s.act}
                    role="button"
                    data-testid={`spot-item-${s.act}`}
                    className={`spotitem ${i === sel ? 'sel' : ''}`}
                    onMouseEnter={() => setSel(i)}
                    onClick={exec}
                  >
                    <span style={{ color: 'var(--text-3)', display: 'inline-flex' }}>
                      <Glyph size={18} />
                    </span>
                    <div style={{ flex: 1 }}>
                      <div style={{ fontSize: 14, fontWeight: 540 }}>
                        {s.label}
                        {showTitle && (
                          <>
                            : <span className="spot-title">{title}</span>
                          </>
                        )}
                      </div>
                      <div className="faint" style={{ fontSize: 12 }}>
                        {s.desc}
                      </div>
                    </div>
                    {i === sel && (
                      <span className="chip mono" style={{ fontSize: 10 }}>
                        ↵
                      </span>
                    )}
                  </div>
                );
              })
            )}
          </div>
          {/* dica à-prova-de-culpa: comando que exige contexto sem projeto (CA-15) */}
          {parsedAct && parsedAct !== 'timer' && !projectId && (
            <div className="faint" data-testid="spot-hint" style={{ padding: '4px 16px 14px', fontSize: 12 }}>
              Abra um projeto pra criar aqui.
            </div>
          )}
        </div>
      </CenterPanel>
    </>
  );
}

// BoardGate (27-H3/CA-16.1): componente-FILHO condicional (não hook condicional)
// — só monta com projectId presente, então o useBoard nunca dispara
// GET /projects//board malformado fora de /projetos/:id nem com o Spotlight
// fechado (o Spotlight inteiro só monta com open=true). Eleva o can_write
// resolvido pro pai via callback.
function BoardGate({
  ws,
  projectId,
  onCanWrite,
}: {
  ws: string;
  projectId: string;
  onCanWrite: (v: boolean) => void;
}) {
  const { data: board } = useBoard(ws, projectId);
  useEffect(() => {
    if (board) onCanWrite(board.project.can_write);
  }, [board, onCanWrite]);
  return null;
}
