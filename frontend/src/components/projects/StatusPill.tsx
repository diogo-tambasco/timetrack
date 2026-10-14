// Pill de status do projeto (T-16, spec §6): Ativo/Arquivado derivado de
// archived_at — o "Pausado" do protótipo fica fora (não existe no schema).
export default function StatusPill({ archived }: { archived: boolean }) {
  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 6,
        fontSize: 11.5,
        fontWeight: 540,
        padding: '3px 10px',
        borderRadius: 999,
        border: '1px solid var(--border)',
        color: archived ? 'var(--text-3)' : 'var(--text-2)',
        whiteSpace: 'nowrap',
      }}
    >
      <i
        className="tt-dot"
        style={{ width: 6, height: 6, background: archived ? 'var(--text-3)' : 'var(--accent)' }}
      />
      {archived ? 'Arquivado' : 'Ativo'}
    </span>
  );
}
