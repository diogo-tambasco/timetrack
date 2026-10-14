// Workspace.manage.spec — fase 17 T-09/T-10: bloco de gestão Owner-only na
// página /workspace. Owner vê form de convite/adicionar + pendentes + controles
// por membro; member/viewer NÃO veem nada disso (CA-29). Add existente com 404
// user_not_found oferece o convite por email (CA-33).
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import Workspace from "./Workspace";
import { useAuthStore } from "../../lib/auth";
import * as api from "../../lib/api";
import { ApiError } from "../../lib/api";
import type { WorkspaceDetail, WorkspaceRole, Invitation } from "../../lib/api";

vi.mock("../../lib/api", async (importOriginal) => {
  const mod = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...mod,
    getWorkspace: vi.fn(),
    listInvitations: vi.fn(),
    createInvitation: vi.fn(),
    revokeInvitation: vi.fn(),
    addMember: vi.fn(),
    removeMember: vi.fn(),
    changeRole: vi.fn(),
  };
});

const detalhe = (): WorkspaceDetail => ({
  id: "w1",
  name: "Workspace de Ana",
  role: "owner",
  created_at: "2026-06-01T10:00:00Z",
  members: [
    { user_id: "u1", name: "Ana", email: "ana@tt.dev", role: "owner" },
    { user_id: "u2", name: "Carlos", email: "carlos@tt.dev", role: "member" },
  ],
});

const pendente = (over?: Partial<Invitation>): Invitation => ({
  id: "i1",
  email: "cadu@example.com",
  role: "member",
  status: "pending",
  expires_at: "2026-07-01T10:00:00Z",
  created_at: "2026-06-20T10:00:00Z",
  ...over,
});

function setMe(role: WorkspaceRole) {
  useAuthStore.setState({
    status: "authenticated",
    me: {
      user: { id: "u1", email: "ana@tt.dev", name: "Ana", avatar_url: null },
      workspaces: [{ id: "w1", name: "Workspace de Ana", role }],
    },
    activeWorkspaceId: null,
  });
}

function renderPagina() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <Workspace />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  vi.mocked(api.getWorkspace).mockResolvedValue(detalhe());
  vi.mocked(api.listInvitations).mockResolvedValue([pendente()]);
  vi.mocked(api.createInvitation).mockResolvedValue(pendente());
  vi.mocked(api.revokeInvitation).mockResolvedValue(undefined);
  vi.mocked(api.addMember).mockResolvedValue({
    user_id: "u9",
    name: "Bob",
    email: "bob@example.com",
    role: "viewer",
  });
  vi.mocked(api.removeMember).mockResolvedValue(undefined);
  vi.mocked(api.changeRole).mockResolvedValue({
    user_id: "u2",
    role: "viewer",
  });
});

describe("Workspace gestão — visibilidade Owner-only (CA-29)", () => {
  it("owner vê o form de adicionar e os convites pendentes", async () => {
    setMe("owner");
    renderPagina();
    expect(await screen.findByText("Carlos")).toBeInTheDocument();

    expect(screen.getByLabelText("Email da pessoa")).toBeInTheDocument();
    expect(screen.getByText("Adicionar")).toBeInTheDocument();
    expect(await screen.findByText("cadu@example.com")).toBeInTheDocument();
  });

  it.each(["member", "viewer"] as const)(
    "%s NÃO vê nenhum controle de gestão",
    async (role) => {
      setMe(role);
      renderPagina();
      await screen.findByText("Carlos");

      expect(screen.queryByLabelText("Email da pessoa")).toBeNull();
      expect(screen.queryByText("Convites pendentes")).toBeNull();
      expect(screen.queryByText("Adicionar")).toBeNull();
      // não dispara a query de convites pra quem não é owner
      expect(api.listInvitations).not.toHaveBeenCalled();
    },
  );
});

describe("Workspace gestão — convite (CA-30/31)", () => {
  it("enviar convite quando a pessoa não tem conta (404 → fallback)", async () => {
    setMe("owner");
    vi.mocked(api.addMember).mockRejectedValueOnce(
      new ApiError(404, "user_not_found", "usuário não encontrado"),
    );
    renderPagina();
    await screen.findByText("Carlos");

    fireEvent.change(screen.getByLabelText("Email da pessoa"), {
      target: { value: "cadu@example.com" },
    });
    fireEvent.click(screen.getByText("Adicionar"));

    // 404 → oferece o convite por email (CA-33)
    expect(await screen.findByText("Enviar convite")).toBeInTheDocument();
    fireEvent.click(screen.getByText("Enviar convite"));

    await waitFor(() =>
      expect(api.createInvitation).toHaveBeenCalledWith("w1", {
        email: "cadu@example.com",
        role: "member",
      }),
    );
  });

  it("email vazio/sem @ não chama a API", async () => {
    setMe("owner");
    renderPagina();
    await screen.findByText("Carlos");

    // o botão Adicionar fica desabilitado sem email válido
    fireEvent.change(screen.getByLabelText("Email da pessoa"), {
      target: { value: "semarroba" },
    });
    expect((screen.getByText("Adicionar") as HTMLButtonElement).disabled).toBe(
      true,
    );
    expect(api.addMember).not.toHaveBeenCalled();
    expect(api.createInvitation).not.toHaveBeenCalled();
  });

  it("revogar um pendente chama revokeInvitation", async () => {
    setMe("owner");
    renderPagina();
    await screen.findByText("cadu@example.com");

    fireEvent.click(
      screen.getByLabelText("Revogar convite de cadu@example.com"),
    );
    await waitFor(() =>
      expect(api.revokeInvitation).toHaveBeenCalledWith("w1", "i1"),
    );
  });

  it("add existente chama addMember na hora", async () => {
    setMe("owner");
    renderPagina();
    await screen.findByText("Carlos");

    fireEvent.change(screen.getByLabelText("Email da pessoa"), {
      target: { value: "bob@example.com" },
    });
    fireEvent.click(screen.getByText("Adicionar"));

    await waitFor(() =>
      expect(api.addMember).toHaveBeenCalledWith("w1", {
        email: "bob@example.com",
        role: "member",
      }),
    );
    expect(api.createInvitation).not.toHaveBeenCalled();
  });
});

describe("Workspace gestão — membros (CA-32)", () => {
  it("trocar a role de um membro chama changeRole", async () => {
    setMe("owner");
    renderPagina();
    await screen.findByText("Carlos");

    fireEvent.change(screen.getByLabelText("Role de u2"), {
      target: { value: "viewer" },
    });
    await waitFor(() =>
      expect(api.changeRole).toHaveBeenCalledWith("w1", "u2", "viewer"),
    );
  });

  it("remover um membro chama removeMember (com confirmação)", async () => {
    setMe("owner");
    vi.spyOn(window, "confirm").mockReturnValue(true);
    renderPagina();
    await screen.findByText("Carlos");

    fireEvent.click(screen.getByLabelText("Remover u2"));
    await waitFor(() =>
      expect(api.removeMember).toHaveBeenCalledWith("w1", "u2"),
    );
  });

  it("o ÚNICO owner não expõe remover nem permite rebaixar (CA-32)", async () => {
    // detalhe com um só owner (u1) → o select dele é desabilitado e não há
    // botão remover pra ele.
    setMe("owner");
    renderPagina();
    await screen.findByText("Carlos");

    // u1 é o único owner: select desabilitado, sem "Remover u1".
    expect(
      (screen.getByLabelText("Role de u1") as HTMLSelectElement).disabled,
    ).toBe(true);
    expect(screen.queryByLabelText("Remover u1")).toBeNull();
    // u2 (member) tem os controles normais.
    expect(
      (screen.getByLabelText("Role de u2") as HTMLSelectElement).disabled,
    ).toBe(false);
    expect(screen.getByLabelText("Remover u2")).toBeInTheDocument();
  });
});
