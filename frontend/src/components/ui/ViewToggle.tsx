// ViewToggle (T-14, CA-27 / fase 24 CA-16): segmented control de N visões. A
// preferência persiste por página via chave de localStorage. O useView genérico
// (fase 24) suporta as 3 strings da aba Tasks (board/list/table) E os 2 valores
// dos consumidores legados (cards/table).
import { useState } from 'react';
import { Icon } from '../icons';

export type View = 'cards' | 'table';

// useView<T> lê a preferência salva na montagem (com mapeamento OPCIONAL do legado
// — CA-16.1) e persiste cada troca; valor desconhecido cai no fallback.
// A aba Tasks (fase 24) passa o mapa legado da chave tt.view.board (CA-16.1):
// 'cards'→'board' (board de colunas) e 'table'→'list' (a LISTA TaskRow legada). O
// legado mapeia o VALOR ANTIGO; o `persist` resolve a COLISÃO do valor novo que
// coincide com uma chave legada: a Table NATIVA é gravada sob um token próprio
// (`table-v2`), decodificado ANTES do legado → o 'table' legado segue → 'list', e a
// Table nativa persiste de verdade (RF-03). Os consumidores legados (Clients/Projects,
// 2 valores cards/table) NÃO passam mapa nem persist → seguem byte-idênticos.
export function useView<T extends string>(
  storageKey: string,
  options: readonly T[],
  fallback: T,
  legacy?: Record<string, T>,
  persist?: Partial<Record<T, string>>,
): [T, (v: T) => void] {
  const [view, setView] = useState<T>(() => resolveView(storageKey, options, fallback, legacy, persist));
  const change = (v: T) => {
    setView(v);
    // grava o token de persistência do valor (default = o próprio valor); só os
    // valores que COLIDEM com uma chave legada precisam de token distinto.
    localStorage.setItem(storageKey, persist?.[v] ?? v);
  };
  return [view, change];
}

function resolveView<T extends string>(
  storageKey: string,
  options: readonly T[],
  fallback: T,
  legacy?: Record<string, T>,
  persist?: Partial<Record<T, string>>,
): T {
  const saved = localStorage.getItem(storageKey);
  if (saved == null) return fallback;
  // 1) token de persistência PRÓPRIO tem precedência (CA-16.1): decodifica o valor
  // novo gravado por ESTA fase (ex.: 'table-v2' → 'table'), ANTES do legado, pra a
  // Table nativa não cair no mapeamento legado de 'table'.
  if (persist != null) {
    for (const v of options) {
      if (persist[v] != null && persist[v] === saved) return v;
    }
  }
  // 2) mapa legado: 'cards' → 'board'; 'table' LEGADO (fase 12) → 'list'.
  if (legacy != null && saved in legacy) {
    const mapped = legacy[saved];
    if ((options as readonly string[]).includes(mapped)) return mapped;
  }
  // 3) valor novo sem token próprio → usa direto (round-trip de escrita própria).
  if ((options as readonly string[]).includes(saved)) return saved as T;
  return fallback;
}

const opcoes = [
  { value: 'cards', label: 'Cartões', Glyph: Icon.grid },
  { value: 'table', label: 'Tabela', Glyph: Icon.table },
] as const;

export default function ViewToggle({
  view,
  onChange,
}: {
  view: View;
  onChange: (v: View) => void;
}) {
  return (
    <div
      style={{
        display: 'flex',
        background: 'var(--surface-3)',
        borderRadius: 9,
        padding: 2,
        gap: 2,
      }}
    >
      {opcoes.map(({ value, label, Glyph }) => (
        <button
          key={value}
          type="button"
          aria-pressed={view === value}
          onClick={() => onChange(value)}
          style={{
            border: 'none',
            cursor: 'pointer',
            fontFamily: 'var(--font-ui)',
            fontSize: 12.5,
            fontWeight: 540,
            padding: '6px 12px',
            borderRadius: 7,
            display: 'flex',
            alignItems: 'center',
            gap: 6,
            background: view === value ? 'var(--surface-1)' : 'transparent',
            color: view === value ? 'var(--text-1)' : 'var(--text-2)',
          }}
        >
          <Glyph size={14} />
          {label}
        </button>
      ))}
    </div>
  );
}
