// Cartão de cliente (T-15): porte do ClientCard de clients.jsx restrito ao
// schema canônico — nome + nº de projetos + chips dos projetos (spec §6:
// contato, valor/h e status do protótipo ficam fora). Renomear e arquivar
// inline, ligados aos hooks pela página.
import { useState, type FormEvent } from 'react';
import type { Client, Project } from '../../lib/api';
import ProjectChip from '../projects/ProjectChip';

export default function ClientCard({
  client,
  projects,
  canWrite,
  onRename,
  onArchive,
}: {
  client: Client;
  projects: Project[];
  // CA-31: viewer não vê Renomear/Arquivar
  canWrite: boolean;
  // promise da mutação: o editor só fecha no sucesso — falha (400/403)
  // mantém o input aberto pro usuário corrigir
  onRename: (name: string) => Promise<unknown>;
  onArchive: () => void;
}) {
  const [editando, setEditando] = useState(false);
  const [nome, setNome] = useState(client.name);

  const abrirEditor = () => {
    setNome(client.name); // re-semeia do valor atual (não do último rascunho)
    setEditando(true);
  };

  const salvar = (e: FormEvent) => {
    e.preventDefault();
    const valor = nome.trim();
    if (!valor) return;
    void onRename(valor).then(
      () => setEditando(false),
      () => {}, // falha: editor segue aberto com o valor digitado
    );
  };

  return (
    <div
      className="tt-card"
      style={{
        padding: 18,
        display: 'flex',
        flexDirection: 'column',
        gap: 14,
        background: 'var(--surface-1)',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12 }}>
        <span
          aria-hidden
          style={{
            width: 38,
            height: 38,
            borderRadius: 11,
            background: 'var(--accent-soft)',
            color: 'var(--accent)',
            flex: '0 0 auto',
            display: 'grid',
            placeItems: 'center',
            fontWeight: 700,
            fontSize: 15,
          }}
        >
          {client.name[0]}
        </span>
        <div style={{ flex: 1, minWidth: 0 }}>
          {editando ? (
            <form onSubmit={salvar} style={{ display: 'flex', gap: 6 }}>
              <input
                aria-label="Novo nome"
                value={nome}
                onChange={(e) => setNome(e.target.value)}
                autoFocus
                style={{
                  flex: 1,
                  minWidth: 0,
                  background: 'var(--surface-2)',
                  border: '1px solid var(--border-2)',
                  borderRadius: 8,
                  color: 'var(--text-1)',
                  font: 'inherit',
                  padding: '4px 8px',
                }}
              />
              <button type="submit" className="tt-btn tt-btn--accent">
                Salvar
              </button>
            </form>
          ) : (
            <div style={{ fontSize: 15, fontWeight: 600 }}>{client.name}</div>
          )}
        </div>
        {canWrite && (
          <div style={{ display: 'flex', gap: 4 }}>
            <button type="button" className="tt-btn tt-btn--ghost" onClick={abrirEditor}>
              Renomear
            </button>
            <button type="button" className="tt-btn tt-btn--ghost" onClick={onArchive}>
              Arquivar
            </button>
          </div>
        )}
      </div>
      <div
        style={{
          borderTop: '1px solid var(--border)',
          paddingTop: 12,
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          flexWrap: 'wrap',
        }}
      >
        <span className="faint" style={{ fontSize: 11.5 }}>
          {projects.length} projeto{projects.length === 1 ? '' : 's'}
        </span>
        <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
          {projects.map((p) => (
            // horas reais só na fase 03 — até lá o chip recebe 0 (CA-30)
            <ProjectChip
              key={p.id}
              name={p.name}
              color={p.color}
              hours={0}
              goal={p.weekly_hours_goal}
            />
          ))}
        </div>
      </div>
    </div>
  );
}
