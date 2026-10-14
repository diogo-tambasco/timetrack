// Guard de autenticação (fase 01 T-14, CA-17a): consulta GET /api/v1/me
// no boot; 401 → /login; 200 → user+workspaces no store. A própria /login
// fica fora do guard (sem loop).
import { useEffect, type ReactNode } from 'react';
import { Navigate } from 'react-router-dom';
import { create } from 'zustand';
import { getMe, logout as apiLogout, ApiError, type Me } from './api';

type AuthStatus = 'loading' | 'authenticated' | 'anonymous';

type AuthState = {
  status: AuthStatus;
  me: Me | null;
  /** workspace ativo (fase 10) — null = comportamento clássico workspaces[0] */
  activeWorkspaceId: string | null;
  load: () => Promise<void>;
  setActiveWorkspace: (id: string) => void;
  signOut: () => Promise<void>;
};

const WS_STORAGE_KEY = 'tt.ws.active';

// lido na CRIAÇÃO do store (boot do app — CA-07); localStorage indisponível
// (Safari private) cai no default em silêncio
function readStoredWorkspaceId(): string | null {
  try {
    return localStorage.getItem(WS_STORAGE_KEY);
  } catch {
    return null;
  }
}

export const useAuthStore = create<AuthState>((set) => ({
  status: 'loading',
  me: null,
  activeWorkspaceId: readStoredWorkspaceId(),

  setActiveWorkspace: (id) => {
    try {
      localStorage.setItem(WS_STORAGE_KEY, id);
    } catch {
      /* persistência indisponível: a troca vale só pra sessão */
    }
    set({ activeWorkspaceId: id });
  },

  load: async () => {
    try {
      const me = await getMe();
      set({ status: 'authenticated', me });
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        set({ status: 'anonymous', me: null });
      } else {
        // rede fora (PWA offline, CA-14 da fase 00): um erro de rede não
        // prova sessão inválida — mantém o shell de pé sem dados de user
        set({ status: 'authenticated', me: null });
      }
    }
  },

  signOut: async () => {
    try {
      await apiLogout();
    } finally {
      set({ status: 'anonymous', me: null });
    }
  },
}));

// useCurrentWorkspace devolve o workspace ATIVO (fase 10): o escolhido no
// seletor (persistido em tt.ws.active) ou, sem escolha/id morto, o
// primeiro do /me (workspaces[0] — ordem por created_at, o pessoal).
export function useCurrentWorkspace() {
  return useAuthStore(
    (s) =>
      s.me?.workspaces.find((w) => w.id === s.activeWorkspaceId) ??
      s.me?.workspaces[0] ??
      null,
  );
}

// useRole (T-13, CA-31): role do usuário no workspace, vinda do GET /me da
// fase 01. Workspace desconhecido (ou sessão sem dados) → null, e a UI
// trata como sem permissão de escrita.
export function useRole(workspaceId?: string): 'owner' | 'member' | 'viewer' | null {
  return useAuthStore((s) => {
    const workspaces = s.me?.workspaces ?? [];
    const ws = workspaceId ? workspaces.find((w) => w.id === workspaceId) : workspaces[0];
    return ws?.role ?? null;
  });
}

// RequireSession embrulha o shell: enquanto não houver sessão válida,
// qualquer rota cai em /login.
export function RequireSession({ children }: { children: ReactNode }) {
  const status = useAuthStore((s) => s.status);
  const load = useAuthStore((s) => s.load);

  useEffect(() => {
    if (status === 'loading') void load();
  }, [status, load]);

  if (status === 'loading') return null; // boot silencioso, sem flash
  if (status === 'anonymous') return <Navigate to="/login" replace />;
  return <>{children}</>;
}
