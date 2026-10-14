// Molde dos stubs da fase 00 — as fases 02+ substituem cada página no
// próprio arquivo dela (Clients.tsx, Projects.tsx...), não aqui.
export default function StubPage({ title }: { title: string }) {
  return (
    <section>
      <p className="eyebrow">{title}</p>
      <div className="tt-card" style={{ marginTop: 'var(--gap)', padding: 'var(--pad)', minHeight: 120 }}>
        <span className="faint">Em construção — chega nas próximas fases.</span>
      </div>
    </section>
  );
}
