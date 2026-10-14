// Tela /login (fase 01 T-13, plan §5): porta de entrada única — marca +
// botão "Entrar com Google". É um <a> com navegação plena (não fetch); o
// cookie httpOnly volta no callback e o guard (T-14) faz o resto.
import { useSearchParams } from 'react-router-dom';

function GoogleLogo() {
  return (
    <svg width="16" height="16" viewBox="0 0 48 48" aria-hidden="true">
      <path
        fill="currentColor"
        d="M44.5 20H24v8.5h11.8C34.7 33.9 30.1 37 24 37c-7.2 0-13-5.8-13-13s5.8-13 13-13c3.1 0 5.9 1.1 8.1 2.9l6.4-6.4C34.6 4.1 29.6 2 24 2 11.8 2 2 11.8 2 24s9.8 22 22 22c11 0 21-8 21-22 0-1.3-.2-2.7-.5-4z"
      />
    </svg>
  );
}

export default function Login() {
  const [params] = useSearchParams();
  const googleError = params.get('error') === 'google';

  return (
    <div
      className="tt"
      style={{ minHeight: '100vh', display: 'grid', placeItems: 'center', background: 'var(--bg)' }}
    >
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 10 }}>
        <span
          aria-hidden="true"
          style={{
            width: 34,
            height: 34,
            borderRadius: 9,
            background: 'var(--accent)',
            display: 'grid',
            placeItems: 'center',
            color: 'var(--accent-ink)',
            font: '700 16px/1 "Geist Mono", monospace',
          }}
        >
          T
        </span>
        <h1 style={{ margin: '6px 0 0', font: '600 26px/1.2 Geist, sans-serif', color: 'var(--text-1)' }}>
          Timetrack
        </h1>
        <p style={{ margin: 0, color: 'var(--text-3)', fontSize: 14 }}>
          Cronometre, planeje e foque — num lugar só.
        </p>

        {googleError && (
          <p style={{ margin: '4px 0 0', color: 'var(--text-2)', fontSize: 13 }}>
            Não foi possível entrar com o Google. Tente de novo.
          </p>
        )}

        <a
          className="tt-btn tt-btn--accent"
          href="/auth/google/login"
          style={{ textDecoration: 'none', marginTop: 14 }}
        >
          <GoogleLogo />
          Entrar com Google
        </a>
      </div>
    </div>
  );
}
