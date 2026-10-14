// Página Workspace (fase 10 — leitura) + gestão Owner-only (fase 17). A
// leitura (nome + membros + seletor de ativo) é IDÊNTICA à fase 10; o bloco de
// gestão (convidar/revogar, trocar role/remover, adicionar existente) só
// renderiza quando useRole(ws.id) === 'owner' (CA-29/34). Member/viewer veem só
// a leitura.
import { useState, type FormEvent } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError, getWorkspace, type WorkspaceRole } from "../../lib/api";
import {
  workspaceKey,
  useInvitations,
  useCreateInvitation,
  useRevokeInvitation,
  useAddMember,
  useRemoveMember,
  useChangeRole,
} from "../../lib/queries/workspace";
import { useAuthStore, useCurrentWorkspace, useRole } from "../../lib/auth";
import PresenceStrip from "../../components/PresenceStrip";

const ROLE_LABEL: Record<WorkspaceRole, string> = {
  owner: "Dono",
  member: "Membro",
  viewer: "Viewer",
};

const inputStyle = {
  flex: 1,
  background: "var(--surface-2)",
  border: "1px solid var(--border-2)",
  borderRadius: 8,
  color: "var(--text-1)",
  font: "inherit" as const,
  padding: "8px 10px",
};

export default function Workspace() {
  const ws = useCurrentWorkspace();
  const workspaces = useAuthStore((s) => s.me?.workspaces ?? []);
  const setActiveWorkspace = useAuthStore((s) => s.setActiveWorkspace);
  const qc = useQueryClient();
  const isOwner = useRole(ws?.id) === "owner";

  // nome + members da MESMA resposta (D-05 — 1 fonte, 1 query; mesma
  // queryKey do useWorkspaceMembers: cache compartilhado)
  const detailQ = useQuery({
    queryKey: workspaceKey(ws?.id ?? ""),
    queryFn: () => getWorkspace(ws?.id ?? ""),
    enabled: ws !== null,
  });
  const detail = detailQ.data;

  return (
    <section>
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          marginBottom: "var(--gap)",
        }}
      >
        <p className="eyebrow" style={{ margin: 0 }}>
          Workspace
        </p>
        {workspaces.length > 1 && (
          <select
            aria-label="Workspace ativo"
            value={ws?.id ?? ""}
            onChange={(e) => {
              setActiveWorkspace(e.target.value);
              // CA-08/D-03: nada do contexto anterior sobrevive na tela
              void qc.invalidateQueries();
            }}
            style={{
              background: "var(--surface-2)",
              color: "var(--text-2)",
              border: "1px solid var(--border)",
              borderRadius: 8,
              padding: "6px 9px",
              fontFamily: "var(--font-ui)",
              fontSize: 12.5,
              maxWidth: 220,
            }}
          >
            {workspaces.map((w) => (
              <option key={w.id} value={w.id}>
                {w.name}
              </option>
            ))}
          </select>
        )}
      </div>

      {/* fase 21: faixa de presence ao vivo (aditiva — não some os blocos da
          fase 10/17). Some sozinha quando ninguém está ativo. */}
      <div style={{ marginBottom: "var(--gap)" }}>
        <PresenceStrip wsId={ws?.id} />
      </div>

      <div className="tt-card" style={{ padding: "var(--pad)" }}>
        <h2
          data-testid="workspace-name"
          style={{
            margin: "0 0 14px",
            fontSize: 17,
            fontWeight: 600,
            letterSpacing: "-.01em",
          }}
        >
          {detail?.name ?? ws?.name ?? "—"}
        </h2>

        <p className="eyebrow">Membros</p>
        <div style={{ display: "grid" }}>
          {(detail?.members ?? []).map((m) => (
            <div
              key={m.user_id}
              data-testid={`member-${m.user_id}`}
              // fase 22 (CA-10): < md a linha quebra (controles do owner caem pra
              // baixo) e a altura passa a auto, pra não cortar nem estourar. ≥ md
              // = altura fixa var(--row-h) de hoje. height/wrap saem do inline.
              className="tt-ws-member"
              style={{
                display: "flex",
                alignItems: "center",
                gap: 12,
                borderBottom: "1px solid var(--border)",
              }}
            >
              <span
                className="tt-avatar"
                style={{
                  background: "var(--surface-3)",
                  color: "var(--text-2)",
                }}
              >
                {(m.name || m.email).trim().charAt(0).toUpperCase()}
              </span>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontSize: "var(--fs-row)", fontWeight: 500 }}>
                  {m.name}
                </div>
                <div className="faint" style={{ fontSize: 11.5 }}>
                  {m.email}
                </div>
              </div>
              {/* leitura (CA-34): o badge de role segue idêntico à fase 10 pra
                  TODOS — member/viewer veem só ele; o owner ganha os controles
                  de gestão ao lado (CA-32). */}
              {isOwner ? (
                <MemberControls
                  ws={ws?.id ?? ""}
                  userId={m.user_id}
                  role={m.role}
                  ownerCount={
                    (detail?.members ?? []).filter((x) => x.role === "owner")
                      .length
                  }
                />
              ) : (
                <span
                  className={
                    m.role === "owner" ? "tt-role tt-role--owner" : "tt-role"
                  }
                >
                  {ROLE_LABEL[m.role]}
                </span>
              )}
            </div>
          ))}
          {detail && detail.members.length === 0 && (
            <div className="muted" style={{ padding: "14px 0", fontSize: 13 }}>
              Só você por aqui.
            </div>
          )}
        </div>
      </div>

      {isOwner && ws && <OwnerManagement ws={ws.id} />}
    </section>
  );
}

// MemberControls (CA-32) — seletor de role + remover por membro. A UI esconde
// a ação de remover/rebaixar do ÚLTIMO owner (backend é a fonte da verdade —
// CA-18/19): com 1 só owner, o seletor dele é desabilitado e não há "remover".
function MemberControls({
  ws,
  userId,
  role,
  ownerCount,
}: {
  ws: string;
  userId: string;
  role: WorkspaceRole;
  ownerCount: number;
}) {
  const changeRole = useChangeRole(ws);
  const removeMember = useRemoveMember(ws);
  const isLastOwner = role === "owner" && ownerCount <= 1;

  return (
    <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
      <select
        aria-label={`Role de ${userId}`}
        value={role}
        disabled={isLastOwner || changeRole.isPending}
        onChange={(e) =>
          changeRole.mutate({ userId, role: e.target.value as WorkspaceRole })
        }
        style={{
          background: "var(--surface-2)",
          color: "var(--text-2)",
          border: "1px solid var(--border)",
          borderRadius: 8,
          padding: "4px 7px",
          fontFamily: "var(--font-ui)",
          fontSize: 12,
        }}
      >
        <option value="owner">Dono</option>
        <option value="member">Membro</option>
        <option value="viewer">Viewer</option>
      </select>
      {!isLastOwner && (
        <button
          type="button"
          className="tt-btn tt-btn--ghost"
          aria-label={`Remover ${userId}`}
          onClick={() => {
            if (window.confirm("Remover este membro do workspace?")) {
              removeMember.mutate(userId);
            }
          }}
        >
          Remover
        </button>
      )}
    </div>
  );
}

// OwnerManagement (CA-30/31/33) — form de convite/adicionar + lista de
// pendentes com revogar. Só renderiza pro owner.
function OwnerManagement({ ws }: { ws: string }) {
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<"member" | "viewer">("member");
  const [msg, setMsg] = useState<string | null>(null);
  const [offerInvite, setOfferInvite] = useState(false);

  const invitesQ = useInvitations(ws, { enabled: true });
  const createInvitation = useCreateInvitation(ws);
  const addMember = useAddMember(ws);
  const revokeInvitation = useRevokeInvitation(ws);

  const emailValido = /.+@.+/.test(email.trim());

  // fluxo único (CA-33): tenta adicionar existente; 404 user_not_found →
  // oferece enviar convite por email.
  async function submitAdd(e: FormEvent) {
    e.preventDefault();
    setMsg(null);
    setOfferInvite(false);
    if (!emailValido) {
      setMsg("Informe um email válido.");
      return;
    }
    try {
      await addMember.mutateAsync({ email: email.trim(), role });
      setEmail("");
      setMsg("Membro adicionado.");
    } catch (err) {
      if (err instanceof ApiError && err.code === "user_not_found") {
        // ainda não tem conta → oferece o convite por email (CA-33)
        setOfferInvite(true);
        setMsg("Essa pessoa ainda não tem conta — enviar convite por email?");
      } else if (err instanceof ApiError) {
        setMsg(err.message);
      } else {
        setMsg("Não foi possível adicionar.");
      }
    }
  }

  async function enviarConvite() {
    setMsg(null);
    if (!emailValido) {
      setMsg("Informe um email válido.");
      return;
    }
    try {
      await createInvitation.mutateAsync({ email: email.trim(), role });
      setEmail("");
      setOfferInvite(false);
      setMsg("Convite enviado.");
    } catch (err) {
      setMsg(
        err instanceof ApiError ? err.message : "Não foi possível convidar.",
      );
    }
  }

  const pendentes = invitesQ.data ?? [];

  return (
    <div
      className="tt-card"
      style={{ padding: "var(--pad)", marginTop: "var(--gap)" }}
    >
      <p className="eyebrow">Adicionar pessoa</p>
      <form
        onSubmit={submitAdd}
        style={{
          display: "flex",
          gap: 10,
          alignItems: "center",
          flexWrap: "wrap",
        }}
      >
        <input
          aria-label="Email da pessoa"
          type="email"
          value={email}
          onChange={(e) => {
            setEmail(e.target.value);
            setOfferInvite(false);
            setMsg(null);
          }}
          placeholder="email@exemplo.com"
          style={inputStyle}
        />
        <select
          aria-label="Role do convite"
          value={role}
          onChange={(e) => setRole(e.target.value as "member" | "viewer")}
          style={{
            background: "var(--surface-2)",
            color: "var(--text-2)",
            border: "1px solid var(--border)",
            borderRadius: 8,
            padding: "8px 9px",
            fontFamily: "var(--font-ui)",
            fontSize: 12.5,
          }}
        >
          <option value="member">Membro</option>
          <option value="viewer">Viewer</option>
        </select>
        <button
          type="submit"
          className="tt-btn tt-btn--accent"
          disabled={!emailValido}
        >
          Adicionar
        </button>
        {offerInvite && (
          <button
            type="button"
            className="tt-btn tt-btn--ghost"
            onClick={enviarConvite}
          >
            Enviar convite
          </button>
        )}
      </form>
      {msg && (
        <div
          data-testid="manage-msg"
          className="faint"
          style={{ marginTop: 8, fontSize: 12.5 }}
        >
          {msg}
        </div>
      )}

      <p className="eyebrow" style={{ marginTop: "var(--gap)" }}>
        Convites pendentes
      </p>
      {pendentes.length === 0 ? (
        <div className="muted" style={{ padding: "8px 0", fontSize: 13 }}>
          Nenhum convite pendente
        </div>
      ) : (
        <div style={{ display: "grid" }}>
          {pendentes.map((inv) => (
            <div
              key={inv.id}
              data-testid={`invite-${inv.id}`}
              style={{
                display: "flex",
                alignItems: "center",
                gap: 12,
                height: "var(--row-h)",
                borderBottom: "1px solid var(--border)",
              }}
            >
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontSize: "var(--fs-row)", fontWeight: 500 }}>
                  {inv.email}
                </div>
                <div className="faint" style={{ fontSize: 11.5 }}>
                  {ROLE_LABEL[inv.role]}
                  {inv.expires_at
                    ? ` · expira em ${new Date(inv.expires_at).toLocaleDateString("pt-BR")}`
                    : ""}
                </div>
              </div>
              <button
                type="button"
                className="tt-btn tt-btn--ghost"
                aria-label={`Revogar convite de ${inv.email}`}
                onClick={() => revokeInvitation.mutate(inv.id)}
              >
                Revogar
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
