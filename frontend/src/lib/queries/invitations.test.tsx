// invitations.test.tsx — fase 17 T-08: os clientes de API batem nos paths
// certos e os hooks de mutação invalidam invitationsKey/workspaceKey (CA-30..33
// refletem sem reload).
import { describe, it, expect, vi, afterEach, type Mock } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import {
  listInvitations,
  createInvitation,
  revokeInvitation,
  addMember,
  removeMember,
  changeRole,
} from "../api";
import {
  invitationsKey,
  workspaceKey,
  useCreateInvitation,
  useRevokeInvitation,
  useAddMember,
  useRemoveMember,
  useChangeRole,
} from "./workspace";

afterEach(() => vi.unstubAllGlobals());

function stubFetch(status = 200, body: unknown = {}): Mock {
  const fetchMock = vi.fn(
    async () =>
      new Response(status === 204 ? null : JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
  );
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

describe("lib/api convites/membros (T-08)", () => {
  it("listInvitations desembrulha { invitations } no path do workspace", async () => {
    const fetchMock = stubFetch(200, { invitations: [{ id: "i1" }] });
    const list = await listInvitations("w1");
    expect(list).toHaveLength(1);
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/workspaces/w1/invitations",
      expect.objectContaining({ credentials: "include" }),
    );
  });

  it("createInvitation faz POST com email+role", async () => {
    const fetchMock = stubFetch(201, { id: "i1" });
    await createInvitation("w1", { email: "cadu@x.com", role: "member" });
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/v1/workspaces/w1/invitations");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual({
      email: "cadu@x.com",
      role: "member",
    });
  });

  it("revokeInvitation faz DELETE (204 sem corpo)", async () => {
    const fetchMock = stubFetch(204);
    await expect(revokeInvitation("w1", "i1")).resolves.toBeUndefined();
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/workspaces/w1/invitations/i1",
      expect.objectContaining({ method: "DELETE" }),
    );
  });

  it("addMember faz POST em /members", async () => {
    const fetchMock = stubFetch(201, {
      user_id: "u1",
      name: "Bob",
      email: "b@x.com",
      role: "viewer",
    });
    await addMember("w1", { email: "b@x.com", role: "viewer" });
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/v1/workspaces/w1/members");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual({ email: "b@x.com", role: "viewer" });
  });

  it("removeMember faz DELETE em /members/{id}", async () => {
    const fetchMock = stubFetch(204);
    await removeMember("w1", "u1");
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/workspaces/w1/members/u1",
      expect.objectContaining({ method: "DELETE" }),
    );
  });

  it("changeRole faz PATCH com { role }", async () => {
    const fetchMock = stubFetch(200, { user_id: "u1", role: "owner" });
    await changeRole("w1", "u1", "owner");
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/v1/workspaces/w1/members/u1");
    expect(init.method).toBe("PATCH");
    expect(JSON.parse(init.body)).toEqual({ role: "owner" });
  });
});

describe("hooks de convite/membro invalidam as queries certas (T-08)", () => {
  function wrapperWith(qc: QueryClient) {
    return ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );
  }

  it.each([
    [
      "createInvitation",
      () => useCreateInvitation("w1"),
      () => ({ email: "c@x.com", role: "member" }),
      () => invitationsKey("w1"),
    ],
    [
      "revokeInvitation",
      () => useRevokeInvitation("w1"),
      () => "i1",
      () => invitationsKey("w1"),
    ],
    [
      "addMember",
      () => useAddMember("w1"),
      () => ({ email: "b@x.com", role: "viewer" }),
      () => workspaceKey("w1"),
    ],
    [
      "removeMember",
      () => useRemoveMember("w1"),
      () => "u1",
      () => workspaceKey("w1"),
    ],
    [
      "changeRole",
      () => useChangeRole("w1"),
      () => ({ userId: "u1", role: "member" }),
      () => workspaceKey("w1"),
    ],
  ] as const)(
    "%s invalida a query certa",
    async (_name, useHook, makeVars, makeKey) => {
      stubFetch(200, { id: "i1" });
      const qc = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      });
      const invalidate = vi.spyOn(qc, "invalidateQueries");
      const { result } = renderHook(() => useHook(), {
        wrapper: wrapperWith(qc),
      });
      await result.current.mutateAsync(makeVars() as never);
      await waitFor(() =>
        expect(invalidate).toHaveBeenCalledWith({ queryKey: makeKey() }),
      );
    },
  );
});
