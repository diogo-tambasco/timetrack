// TaskThread (fase 19 T-08, plan §6): painel da thread de comentários no
// detalhe da task. Lista autor + timestamp + corpo (ASC, o backend já entrega
// ordenado); input + "Comentar" (desabilitado se body vazio ou !canComment);
// ✕ apaga o PRÓPRIO (compara author_id ao user atual) e, se owner do ws,
// qualquer um (gating de UI — a autoridade é o backend, CA-08/09). Sem
// polling (tempo real fora de escopo).
import { useState, type KeyboardEvent } from 'react';
import { fmtDateTime } from '../../lib/format';
import { useAuthStore, useRole } from '../../lib/auth';
import { Icon } from '../icons';
import {
  useComments,
  useCreateComment,
  useDeleteComment,
} from '../../lib/queries/comments';

export default function TaskThread({
  wsId,
  taskId,
  canComment,
}: {
  wsId: string;
  taskId: string;
  canComment: boolean;
}) {
  const currentUserId = useAuthStore((s) => s.me?.user.id ?? null);
  const isOwner = useRole(wsId) === 'owner';

  const { data: comments, isLoading, isError } = useComments(wsId, taskId);
  const criar = useCreateComment(wsId, taskId);
  const apagar = useDeleteComment(wsId, taskId);

  const [texto, setTexto] = useState('');
  const trimmed = texto.trim();

  // NÃO usamos <form> aqui: a thread vive DENTRO do <form> do TaskModal e
  // <form> aninhado é HTML inválido (o submit borbulharia e salvaria a task).
  // O botão é type="button" e Cmd/Ctrl+Enter envia.
  const submit = () => {
    if (!trimmed || !canComment) return;
    criar.mutate(trimmed, { onSuccess: () => setTexto('') });
  };

  const onKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
      e.preventDefault();
      submit();
    }
  };

  return (
    <section
      aria-label="Comentários"
      style={{
        borderTop: '1px solid var(--border)',
        padding: '16px 20px',
        display: 'flex',
        flexDirection: 'column',
        gap: 12,
      }}
    >
      <div style={{ fontSize: 12.5, fontWeight: 600 }} className="faint">
        Comentários
      </div>

      <ol
        role="log"
        aria-label="Thread de comentários"
        style={{ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: 10 }}
      >
        {isLoading && <li className="faint" style={{ fontSize: 12.5 }}>Carregando…</li>}
        {isError && (
          <li role="alert" style={{ fontSize: 12.5, color: 'var(--danger)' }}>
            Não foi possível carregar os comentários.
          </li>
        )}
        {!isLoading && !isError && comments?.length === 0 && (
          <li className="faint" style={{ fontSize: 12.5 }}>Nenhum comentário ainda.</li>
        )}
        {comments?.map((c) => {
          const podeApagar = c.author_id === currentUserId || isOwner;
          return (
            <li
              key={c.id}
              data-testid="comment"
              style={{
                background: 'var(--surface-2)',
                border: '1px solid var(--border-2)',
                borderRadius: 8,
                padding: '8px 10px',
                display: 'flex',
                flexDirection: 'column',
                gap: 4,
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8 }}>
                <span style={{ fontSize: 12.5, fontWeight: 600 }}>{c.author_name}</span>
                <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                  <time className="faint" style={{ fontSize: 11.5 }} dateTime={c.created_at}>
                    {fmtDateTime(c.created_at)}
                  </time>
                  {podeApagar && (
                    <button
                      type="button"
                      aria-label="Apagar comentário"
                      className="tt-iconbtn"
                      onClick={() => apagar.mutate(c.id)}
                      style={{ width: 24, height: 24, background: 'transparent', color: 'var(--text-3)' }}
                    >
                      <Icon.trash size={13} />
                    </button>
                  )}
                </span>
              </div>
              <p style={{ margin: 0, fontSize: 13, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>{c.body}</p>
            </li>
          );
        })}
      </ol>

      {canComment && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          <textarea
            aria-label="Novo comentário"
            value={texto}
            onChange={(e) => setTexto(e.target.value)}
            onKeyDown={onKeyDown}
            rows={2}
            placeholder="Escreva um comentário…"
            style={{
              background: 'var(--surface-2)',
              border: '1px solid var(--border-2)',
              borderRadius: 8,
              color: 'var(--text-1)',
              font: 'inherit',
              fontSize: 13,
              padding: '8px 10px',
              width: '100%',
              resize: 'vertical',
            }}
          />
          <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <button
              type="button"
              className="tt-btn tt-btn--accent"
              onClick={submit}
              disabled={!trimmed || criar.isPending}
            >
              Comentar
            </button>
          </div>
        </div>
      )}
    </section>
  );
}
