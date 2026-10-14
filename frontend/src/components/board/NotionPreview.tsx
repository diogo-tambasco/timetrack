// NotionPreview (fase 24 T-13 → fase 31): o link da task em DUAS superfícies.
//  • NotionField  = a PROPRIEDADE "Link": chip MÍNIMO (ícone + nome do issue/PR/
//    página, efeito de redirect no hover) — não cobre a linha. Vazio → input.
//  • ReferenceCard = o CARD de referência rico (estilo embed do Notion/GitHub),
//    montado LOGO ACIMA da descrição e RETRÁTIL (fase 31). Mark "N" via tokens
//    --notion-mark-bg/ink (ZERO hex literal). Gating: !canWrite → preview-only.
import { useRef, useState } from 'react';
import type { BoardTask } from '../../lib/api';
import { Icon } from '../icons';
import { linkInfo, parseLinks, joinLinks, type LinkKind } from '../../lib/linkPreview';

function NotionMark({ size = 18 }: { size?: number }) {
  return (
    <span
      aria-hidden
      data-testid="notion-mark"
      style={{
        width: size,
        height: size,
        borderRadius: size >= 32 ? 9 : 6,
        background: 'var(--notion-mark-bg)',
        color: 'var(--notion-mark-ink)',
        display: 'grid',
        placeItems: 'center',
        fontWeight: 800,
        fontSize: Math.round(size * 0.6),
        flex: '0 0 auto',
        fontFamily: "Georgia, 'Times New Roman', serif",
        lineHeight: 1,
      }}
    >
      N
    </span>
  );
}

// ícone-box do link: GitHub = octocat · Notion = "N" (marcas próprias, mantidas).
// Os DEMAIS = o FAVICON real do site (reddit/x/youtube/linkedin/…), servido pelo
// serviço de favicons do Google; offline/erro → globo. Escala com `size` (18→38).
function LinkMark({ url, kind, size = 28 }: { url: string; kind: LinkKind; size?: number }) {
  const [failed, setFailed] = useState(false);
  const icon = Math.round(size * 0.6);
  const box = (children: React.ReactNode) => (
    <span
      aria-hidden
      data-testid="link-mark"
      style={{
        width: size,
        height: size,
        borderRadius: size >= 32 ? 9 : 7,
        background: 'var(--surface-3)',
        border: '1px solid var(--border)',
        display: 'grid',
        placeItems: 'center',
        color: 'var(--text-1)',
        flex: '0 0 auto',
        overflow: 'hidden',
      }}
    >
      {children}
    </span>
  );

  // marcas próprias (o Diogo quer manter)
  if (kind === 'github') return box(<Icon.github size={icon} />);
  if (kind === 'notion') return <NotionMark size={size} />;

  // genérico → favicon do domínio; fallback globo
  let host = '';
  try {
    host = new URL(url).hostname;
  } catch {
    /* url inválida → sem favicon */
  }
  const favicon = host ? `https://www.google.com/s2/favicons?sz=64&domain=${host}` : '';
  if (favicon && !failed) {
    return box(
      <img src={favicon} alt="" width={icon} height={icon} loading="lazy" onError={() => setFailed(true)} style={{ display: 'block' }} />,
    );
  }
  return box(<Icon.globe size={icon} />);
}

// ── LinkEmbed: um card de referência rico (ícone + título + subtítulo + abrir) ─
// O card inteiro redireciona (nova aba). Pode haver vários (Notion + GitHub…).
function LinkEmbed({ url }: { url: string }) {
  const info = linkInfo(url);
  return (
    <a
      data-testid="link-embed"
      href={url}
      target="_blank"
      rel="noreferrer"
      aria-label={`Abrir no ${info.service}`}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 12,
        padding: '11px 13px',
        border: '1px solid var(--border-2)',
        borderRadius: 12,
        background: 'var(--surface-2)',
        textDecoration: 'none',
        color: 'var(--text-1)',
      }}
    >
      <LinkMark url={url} kind={info.kind} size={38} />
      <span style={{ flex: 1, minWidth: 0 }}>
        <span style={{ display: 'block', fontSize: 14, fontWeight: 600, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
          {info.title}
        </span>
        <span className="faint" style={{ display: 'block', fontSize: 12, marginTop: 1 }}>
          {info.subtitle}
        </span>
      </span>
      <Icon.ext size={15} style={{ color: 'var(--text-3)', flex: '0 0 auto' }} />
    </a>
  );
}

// ── ReferenceLinks: disclosure "Links" (fechado por padrão, DISCRETO) acima da
// descrição. Não é o foco — só sinaliza que há links pra ver. Suporta vários
// (Notion + GitHub + …). Expandir mostra os cards (LinkEmbed) empilhados.
export function ReferenceLinks({ urls }: { urls: string[] }) {
  const [open, setOpen] = useState(false);
  if (urls.length === 0) return null;
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
        className="faint"
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          gap: 4,
          alignSelf: 'flex-start',
          padding: 0,
          margin: 0,
          border: 'none',
          background: 'transparent',
          font: 'inherit',
          fontSize: 11.5,
          cursor: 'pointer',
        }}
      >
        <Icon.chevron size={11} style={{ transform: open ? 'none' : 'rotate(-90deg)', transition: 'transform .12s ease' }} />
        <span>Links</span>
        <span style={{ opacity: 0.7 }}>{urls.length}</span>
      </button>
      {open && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {urls.map((u) => (
            <LinkEmbed key={u} url={u} />
          ))}
        </div>
      )}
    </div>
  );
}

// LinkChip: um link na linha da propriedade — chip MÍNIMO (ícone + nome, redirect
// no hover). × desvincula só este (canWrite).
function LinkChip({ url, canWrite, onRemove }: { url: string; canWrite: boolean; onRemove: () => void }) {
  const info = linkInfo(url);
  const [hover, setHover] = useState(false);
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 2, minWidth: 0, maxWidth: '100%' }}>
      <a
        data-testid="notion-preview"
        href={url}
        target="_blank"
        rel="noreferrer"
        aria-label={`Abrir no ${info.service}`}
        onMouseEnter={() => setHover(true)}
        onMouseLeave={() => setHover(false)}
        onFocus={() => setHover(true)}
        onBlur={() => setHover(false)}
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          gap: 7,
          minWidth: 0,
          maxWidth: '100%',
          padding: '2px 7px 2px 4px',
          borderRadius: 7,
          textDecoration: 'none',
          color: 'var(--text-1)',
          fontSize: 13,
          fontWeight: 540,
          background: hover ? 'var(--surface-2)' : 'transparent',
          transition: 'background .12s ease',
        }}
      >
        <LinkMark url={url} kind={info.kind} size={18} />
        <span
          style={{
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
            textDecoration: hover ? 'underline' : 'none',
            textUnderlineOffset: 2,
          }}
        >
          {info.title}
        </span>
        <Icon.ext
          size={12}
          style={{
            color: 'var(--text-3)',
            flex: '0 0 auto',
            opacity: hover ? 1 : 0,
            transform: hover ? 'translate(0,0)' : 'translate(-2px,1px)',
            transition: 'opacity .12s ease, transform .12s ease',
          }}
        />
      </a>
      {canWrite && (
        <button
          type="button"
          aria-label={`Desvincular ${info.service}`}
          className="tt-iconbtn"
          onClick={onRemove}
          style={{ width: 22, height: 22, background: 'transparent', color: 'var(--text-3)', flex: '0 0 auto' }}
        >
          <Icon.x size={12} />
        </button>
      )}
    </span>
  );
}

// ── NotionField: a PROPRIEDADE "Link" — agora MULTI: lista de chips + adicionar
// inline. Vários links cabem no campo (separados por \n — joinLinks/parseLinks).
// Clicar na área à direita dos chips (ou com 0 link) abre um input pra colar mais.
export default function NotionField({
  task,
  canWrite,
  onPatch,
}: {
  task: BoardTask;
  canWrite: boolean;
  onPatch: (notion_url: string | null) => void;
}) {
  const urls = parseLinks(task.notion_url);
  const [draft, setDraft] = useState('');
  const [adding, setAdding] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  if (!canWrite && urls.length === 0) {
    return <span className="faint" style={{ fontSize: 12.5 }}>Sem link.</span>;
  }

  const removeAt = (u: string) => onPatch(joinLinks(urls.filter((x) => x !== u)));
  const openAdder = () => {
    setAdding(true);
    requestAnimationFrame(() => inputRef.current?.focus());
  };
  const commit = () => {
    const v = draft.trim();
    if (v) {
      onPatch(joinLinks([...urls, v]));
      setDraft('');
      // mantém o input aberto e focado pra colar mais de uma vez (estilo Notion)
      requestAnimationFrame(() => inputRef.current?.focus());
    } else {
      setAdding(false);
    }
  };

  // 0 links → mostra o input direto (placeholder convidativo). Com links → chips
  // + área clicável à direita (ou o input quando "adding").
  const showInput = canWrite && (adding || urls.length === 0);

  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap', flex: 1, minWidth: 0 }}>
      {urls.map((u) => (
        <LinkChip key={u} url={u} canWrite={canWrite} onRemove={() => removeAt(u)} />
      ))}

      {showInput && (
        <input
          ref={inputRef}
          aria-label="Link do Notion"
          placeholder={urls.length ? 'Adicionar link…' : 'Cole um link do Notion, GitHub…'}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault();
              commit();
            } else if (e.key === 'Escape') {
              setDraft('');
              setAdding(false);
            }
          }}
          onBlur={commit}
          style={{
            flex: 1,
            minWidth: 120,
            background: 'transparent',
            border: 'none',
            outline: 'none',
            color: 'var(--text-1)',
            font: 'inherit',
            fontSize: 13,
            padding: 0,
          }}
        />
      )}

      {/* área clicável à direita dos chips → adiciona um novo link (mockup: clicar
          ao lado do link existente). Só quando há link E não está digitando. */}
      {canWrite && urls.length > 0 && !adding && (
        <button
          type="button"
          aria-label="Adicionar link"
          title="Adicionar link"
          onClick={openAdder}
          style={{
            flex: 1,
            minWidth: 40,
            alignSelf: 'stretch',
            display: 'flex',
            alignItems: 'center',
            gap: 5,
            padding: '2px 4px',
            border: 'none',
            background: 'transparent',
            color: 'var(--text-3)',
            font: 'inherit',
            fontSize: 12,
            cursor: 'text',
            textAlign: 'left',
            opacity: 0.6,
          }}
        >
          <Icon.plus size={12} sw={2.2} />
        </button>
      )}
    </div>
  );
}

export { NotionMark };
