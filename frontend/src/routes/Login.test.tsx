import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Login from './Login';

// CA-17b: marca + botão único "Entrar com Google" (estilo accent) como
// link pra /auth/google/login; ?error=google exibe erro discreto.
describe('tela /login', () => {
  it('login_marcaEBotaoGoogle', () => {
    render(
      <MemoryRouter initialEntries={['/login']}>
        <Login />
      </MemoryRouter>,
    );

    expect(screen.getByText('Timetrack')).toBeInTheDocument();

    const button = screen.getByRole('link', { name: /entrar com google/i });
    expect(button).toHaveAttribute('href', '/auth/google/login');
    expect(button.className).toContain('tt-btn--accent');

    expect(screen.queryByText(/não foi possível entrar/i)).not.toBeInTheDocument();
  });

  it('login_erroGoogleExibeMensagem', () => {
    render(
      <MemoryRouter initialEntries={['/login?error=google']}>
        <Login />
      </MemoryRouter>,
    );

    expect(screen.getByText(/não foi possível entrar/i)).toBeInTheDocument();
  });
});
